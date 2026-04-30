// Package tool defines the ToolFactory abstraction used to build the catalog
// of gollem.Tool instances exposed to the LLM. Each subdirectory (notion,
// slack, meta, ticket) provides a Factory whose constructor receives only the
// concrete dependencies it needs — there is intentionally no shared "deps"
// bag, since those collapse into junk drawers as features grow.
package tool

import (
	"context"

	"github.com/m-mizutani/gollem"
	"github.com/m-mizutani/shepherd/pkg/domain/types"
	"github.com/urfave/cli/v3"
)

// ProviderID names a category of tools (notion, slack, ticket, meta). Values
// match types.SourceProvider where the category exposes a Source.
type ProviderID string

const (
	ProviderMeta   ProviderID = "meta"
	ProviderTicket ProviderID = "ticket"
	ProviderSlack  ProviderID = "slack"
	ProviderNotion ProviderID = "notion"
)

func (p ProviderID) String() string { return string(p) }

// ToolFactory is the only thing the catalog (and pkg/cli) sees. It is
// intentionally narrow: each implementation knows its own concrete
// dependencies (passed via its constructor) and its own CLI flags.
type ToolFactory interface {
	// ID returns the stable provider identifier used for grouping and gating.
	ID() ProviderID

	// Flags returns provider-specific CLI flags. Values are bound to fields
	// inside the concrete factory via cli.Flag.Destination, so by the time
	// Init runs the factory already owns its parsed flag values.
	Flags() []cli.Flag

	// Init is called once after CLI parsing, before serve starts. The factory
	// uses its already-injected dependencies plus parsed flag values to
	// construct whatever it needs (HTTP clients, guards, the tool slice).
	Init(ctx context.Context) error

	// Available reports whether the factory produced any tools. False means
	// the factory is intentionally inert (e.g. token unset) — not an error.
	Available() bool

	// Tools returns the LLM-facing tool instances. Catalog applies
	// per-workspace toggling on top of this list.
	Tools() []gollem.Tool

	// DefaultEnabled is the initial per-workspace toggle for new workspaces.
	DefaultEnabled() bool

	// Prompt returns provider-level narrative (markdown) suitable for
	// embedding into a planner system prompt. Implementations may include
	// workspace-scoped data (e.g. registered Sources). Returning "" means
	// the provider has no extra narrative beyond per-tool Spec descriptions.
	// The catalog isolates errors per provider, so an error here only blanks
	// out this provider's narrative — the tool list still surfaces.
	Prompt(ctx context.Context, ws types.WorkspaceID) (string, error)
}
