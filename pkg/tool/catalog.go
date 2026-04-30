package tool

import (
	"context"

	"github.com/m-mizutani/goerr/v2"
	"github.com/m-mizutani/gollem"
	"github.com/m-mizutani/shepherd/pkg/domain/interfaces"
	"github.com/m-mizutani/shepherd/pkg/domain/types"
	"github.com/m-mizutani/shepherd/pkg/utils/errutil"
)

// GateFn is an extra precondition applied per workspace, on top of
// Available() and the workspace-level toggle. Returning false hides the
// factory's tools without an error.
type GateFn func(ctx context.Context, ws types.WorkspaceID) (bool, error)

// Catalog gates the registered factories by global Availability, per-workspace
// toggle, and provider-specific GateFn (e.g. Notion requires ≥1 Source).
type Catalog struct {
	factories []ToolFactory
	settings  interfaces.ToolSettingsRepository
	gates     map[ProviderID]GateFn
}

// NewCatalog builds a Catalog. settings may be nil — DefaultEnabled then
// applies for every workspace.
func NewCatalog(factories []ToolFactory, settings interfaces.ToolSettingsRepository) *Catalog {
	return &Catalog{
		factories: factories,
		settings:  settings,
		gates:     make(map[ProviderID]GateFn),
	}
}

// WithGate registers an extra GateFn for a provider. Returns the catalog so
// calls can be chained at startup.
func (c *Catalog) WithGate(p ProviderID, fn GateFn) *Catalog {
	c.gates[p] = fn
	return c
}

// Factories exposes the registered factories (used by the HTTP /tools
// endpoint to render status rows).
func (c *Catalog) Factories() []ToolFactory { return c.factories }

// State is the effective per-workspace status of a single factory.
type State struct {
	ID             ProviderID
	Available      bool
	DefaultEnabled bool
	Enabled        bool
	Reason         string // empty when Enabled is true
}

const (
	ReasonProviderUnavailable = "provider_unavailable"
	ReasonWorkspaceDisabled   = "workspace_disabled"
	ReasonGateBlocked         = "gate_blocked"
)

// States returns the State for every registered factory in the workspace.
func (c *Catalog) States(ctx context.Context, ws types.WorkspaceID) ([]State, error) {
	settings, err := c.loadSettings(ctx, ws)
	if err != nil {
		return nil, err
	}
	out := make([]State, 0, len(c.factories))
	for _, f := range c.factories {
		st := State{
			ID:             f.ID(),
			Available:      f.Available(),
			DefaultEnabled: f.DefaultEnabled(),
		}
		switch {
		case !st.Available:
			st.Reason = ReasonProviderUnavailable
		default:
			gateOK, err := c.evalGate(ctx, ws, f.ID())
			if err != nil {
				return nil, err
			}
			if !gateOK {
				// Gate (e.g. "no Source registered") is checked before the
				// per-workspace toggle because a failing gate is the
				// technical precondition — it would be misleading to tell
				// the user "workspace_disabled" when the real reason is the
				// missing prerequisite.
				st.Reason = ReasonGateBlocked
			} else if !settings.IsEnabled(string(f.ID()), f.DefaultEnabled()) {
				st.Reason = ReasonWorkspaceDisabled
			} else {
				st.Enabled = true
			}
		}
		out = append(out, st)
	}
	return out, nil
}

// For returns the tools visible to the agent for this workspace, after
// applying Available × workspace-toggle × GateFn.
func (c *Catalog) For(ctx context.Context, ws types.WorkspaceID) ([]gollem.Tool, error) {
	states, err := c.States(ctx, ws)
	if err != nil {
		return nil, err
	}
	enabled := make(map[ProviderID]bool, len(states))
	for _, s := range states {
		if s.Enabled {
			enabled[s.ID] = true
		}
	}
	var tools []gollem.Tool
	for _, f := range c.factories {
		if enabled[f.ID()] {
			tools = append(tools, f.Tools()...)
		}
	}
	return tools, nil
}

func (c *Catalog) loadSettings(ctx context.Context, ws types.WorkspaceID) (settingsView, error) {
	if c.settings == nil {
		return settingsView{}, nil
	}
	s, err := c.settings.Get(ctx, ws)
	if err != nil {
		return settingsView{}, goerr.Wrap(err, "failed to load tool settings",
			goerr.V("workspace_id", string(ws)))
	}
	return settingsView{enabled: s.Enabled}, nil
}

func (c *Catalog) evalGate(ctx context.Context, ws types.WorkspaceID, p ProviderID) (bool, error) {
	gate, ok := c.gates[p]
	if !ok {
		return true, nil
	}
	return gate(ctx, ws)
}

// ProviderBriefing is the per-provider data the catalog produces for the
// triage planner system prompt: the provider's narrative (from
// ToolFactory.Prompt) plus its tool list. Only enabled providers — those
// whose State.Enabled is true — appear in the slice.
type ProviderBriefing struct {
	ID          ProviderID
	Description string // markdown narrative; empty when factory returns "" or errors
	Tools       []ToolEntry
}

// ToolEntry is the planner-facing summary of a single gollem.Tool. It mirrors
// the fields the planner needs in order to fill `allowed_tools` correctly.
type ToolEntry struct {
	Name        string
	Description string
}

// ToolBriefing returns enabled providers (Available × workspace toggle ×
// gate) with their narrative and tool listings, in factory registration
// order. Per-provider Prompt() errors are routed through errutil.Handle so
// they reach slog/Sentry, but the provider's tool entries still surface
// with an empty Description — losing one provider's narrative must not
// abort the whole call, since triage relies on this for planner prompts.
func (c *Catalog) ToolBriefing(ctx context.Context, ws types.WorkspaceID) ([]ProviderBriefing, error) {
	states, err := c.States(ctx, ws)
	if err != nil {
		return nil, err
	}
	enabled := make(map[ProviderID]bool, len(states))
	for _, s := range states {
		if s.Enabled {
			enabled[s.ID] = true
		}
	}
	out := make([]ProviderBriefing, 0, len(c.factories))
	for _, f := range c.factories {
		if !enabled[f.ID()] {
			continue
		}
		desc, perr := f.Prompt(ctx, ws)
		if perr != nil {
			errutil.Handle(ctx, goerr.Wrap(perr, "render provider prompt for triage briefing",
				goerr.V("provider_id", string(f.ID())),
				goerr.V("workspace_id", string(ws))))
			desc = ""
		}
		entries := make([]ToolEntry, 0, len(f.Tools()))
		for _, t := range f.Tools() {
			spec := t.Spec()
			entries = append(entries, ToolEntry{
				Name:        spec.Name,
				Description: spec.Description,
			})
		}
		out = append(out, ProviderBriefing{
			ID:          f.ID(),
			Description: desc,
			Tools:       entries,
		})
	}
	return out, nil
}

type settingsView struct {
	enabled map[string]bool
}

func (v settingsView) IsEnabled(providerID string, defaultEnabled bool) bool {
	if v.enabled == nil {
		return defaultEnabled
	}
	if val, ok := v.enabled[providerID]; ok {
		return val
	}
	return defaultEnabled
}
