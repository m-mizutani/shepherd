package prompt

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"text/template"
	"time"

	"github.com/m-mizutani/goerr/v2"
	"github.com/m-mizutani/shepherd/pkg/domain/interfaces"
	"github.com/m-mizutani/shepherd/pkg/domain/model"
	"github.com/m-mizutani/shepherd/pkg/domain/types"
	"github.com/m-mizutani/shepherd/pkg/utils/errutil"
	"github.com/m-mizutani/shepherd/pkg/utils/logging"
)

// ErrInvalidTemplate is returned by UseCase.Save when the supplied content
// either fails to parse as a Go text/template or fails to Execute against
// every registered probe input. Controllers map this to HTTP 422.
var ErrInvalidTemplate = goerr.New("invalid prompt template")

// UseCase manages workspace-level prompt overrides. It is also the single
// place that renders prompts for downstream agents (e.g. triage planner),
// so override lookup and rendering are colocated.
type UseCase struct {
	repo interfaces.PromptRepository
}

// New constructs a UseCase backed by a PromptRepository.
func New(repo interfaces.PromptRepository) *UseCase {
	return &UseCase{repo: repo}
}

// Author identifies who saved a version. Sourced from the auth token in the
// request context by the controller.
type Author struct {
	Name  string
	Email string
	Sub   string
}

// slotDef registers per-PromptID metadata: the embedded default content and
// the probe inputs used to validate user-supplied templates at Save time.
//
// probes must collectively exercise every {{ .Field }} / {{ if }} / {{ range }}
// action in the default template — both the populated and the empty branch —
// so a user who introduces a typo in a field name (caught by missingkey=error)
// or pipelines `range` over the wrong type fails Save instead of triage.
type slotDef struct {
	defaultSource string
	probes        []any
}

var slotDefs = map[model.PromptID]slotDef{
	model.PromptIDTriage: {
		defaultSource: triagePlanTemplateSource,
		probes: []any{
			TriagePlanInput{
				Title:          "Sample title",
				Description:    "Sample description",
				InitialMessage: "Sample initial message",
				Reporter:       "U123",
			},
			TriagePlanInput{Title: "title only"},
		},
	},
}

// Effective returns the content currently in force for (ws, id) along with
// its version number. version == 0 means no override exists yet and the
// returned content is the embedded default.
func (u *UseCase) Effective(ctx context.Context, ws types.WorkspaceID, id model.PromptID) (string, int, error) {
	def, ok := slotDefs[id]
	if !ok {
		return "", 0, goerr.New("unknown prompt id", goerr.V("prompt_id", string(id)))
	}
	if u.repo == nil {
		return def.defaultSource, 0, nil
	}
	cur, err := u.repo.GetCurrent(ctx, ws, id)
	if err != nil {
		return "", 0, goerr.Wrap(err, "load current prompt override",
			goerr.V("workspace_id", string(ws)),
			goerr.V("prompt_id", string(id)))
	}
	if cur == nil {
		return def.defaultSource, 0, nil
	}
	return cur.Content, cur.Version, nil
}

// Default returns the embedded default content for id (used by the UI to
// present a "Discard to default" option and by Effective fallback).
func (u *UseCase) Default(id model.PromptID) (string, error) {
	def, ok := slotDefs[id]
	if !ok {
		return "", goerr.New("unknown prompt id", goerr.V("prompt_id", string(id)))
	}
	return def.defaultSource, nil
}

// Save validates content against the slot's parser + probe inputs and
// atomically appends it as `version`. The version must equal current+1
// (or 1 when no override exists yet); otherwise the call is rejected with
// ErrPromptVersionConflict. ErrInvalidTemplate indicates the user-supplied
// content is unsafe.
func (u *UseCase) Save(ctx context.Context, ws types.WorkspaceID, id model.PromptID, version int, content string, by Author) (*model.PromptVersion, error) {
	if u.repo == nil {
		return nil, goerr.New("prompt repository not configured")
	}
	if _, ok := slotDefs[id]; !ok {
		return nil, goerr.New("unknown prompt id", goerr.V("prompt_id", string(id)))
	}
	if err := validateTemplate(id, content); err != nil {
		return nil, err
	}

	draft := &model.PromptVersion{
		Version:        version,
		Content:        content,
		UpdatedAt:      time.Now(),
		UpdatedBy:      by.Name,
		UpdatedByEmail: by.Email,
		UpdatedBySub:   by.Sub,
	}
	return u.repo.Append(ctx, ws, id, draft)
}

// History returns all versions oldest-first. The last entry is the current
// version.
func (u *UseCase) History(ctx context.Context, ws types.WorkspaceID, id model.PromptID) ([]*model.PromptVersion, error) {
	if u.repo == nil {
		return nil, nil
	}
	if _, ok := slotDefs[id]; !ok {
		return nil, goerr.New("unknown prompt id", goerr.V("prompt_id", string(id)))
	}
	return u.repo.List(ctx, ws, id)
}

// Restore copies the content of targetVersion into a brand-new version
// appended as `version`. Conflict semantics match Save: `version` must equal
// current+1.
func (u *UseCase) Restore(ctx context.Context, ws types.WorkspaceID, id model.PromptID, version int, targetVersion int, by Author) (*model.PromptVersion, error) {
	if u.repo == nil {
		return nil, goerr.New("prompt repository not configured")
	}
	if _, ok := slotDefs[id]; !ok {
		return nil, goerr.New("unknown prompt id", goerr.V("prompt_id", string(id)))
	}
	target, err := u.repo.GetVersion(ctx, ws, id, targetVersion)
	if err != nil {
		return nil, err
	}
	// Restore should not be a back-door for storing previously-valid but
	// now-broken content (slotDefs may have changed). Re-validate.
	if err := validateTemplate(id, target.Content); err != nil {
		return nil, err
	}
	draft := &model.PromptVersion{
		Version:        version,
		Content:        target.Content,
		UpdatedAt:      time.Now(),
		UpdatedBy:      by.Name,
		UpdatedByEmail: by.Email,
		UpdatedBySub:   by.Sub,
	}
	return u.repo.Append(ctx, ws, id, draft)
}

// RenderTriagePlan looks up the effective triage prompt for ws and executes
// it against the supplied input. If override execution fails at runtime, it
// logs and falls back to the embedded default so a broken override never
// stops triage entirely.
func (u *UseCase) RenderTriagePlan(ctx context.Context, ws types.WorkspaceID, in TriagePlanInput) (string, error) {
	content, version, err := u.Effective(ctx, ws, model.PromptIDTriage)
	if err != nil {
		return "", err
	}
	rendered, renderErr := executeTemplate(string(model.PromptIDTriage), content, in)
	if renderErr == nil {
		return rendered, nil
	}
	if version == 0 {
		// The embedded default failed — that is a programming error, not a
		// user override issue. Surface it.
		return "", goerr.Wrap(renderErr, "execute embedded triage template")
	}
	// Override is broken at runtime. Log + Sentry, then fall back so triage
	// keeps working with the embedded default.
	errutil.Handle(ctx,
		goerr.Wrap(renderErr, "broken triage prompt override; falling back to default",
			goerr.V("workspace_id", string(ws)),
			goerr.V("prompt_version", version)))
	logging.From(ctx).Warn("triage prompt override execution failed; using default",
		slog.String("workspace_id", string(ws)),
		slog.Int("prompt_version", version))
	def, _ := slotDefs[model.PromptIDTriage]
	return executeTemplate(string(model.PromptIDTriage), def.defaultSource, in)
}

// validateTemplate parses content with missingkey=error and Executes it
// against every registered probe for id. Any failure is wrapped with
// ErrInvalidTemplate and the underlying message is exposed via goerr.V so
// the controller can include it in the 422 response.
func validateTemplate(id model.PromptID, content string) error {
	def, ok := slotDefs[id]
	if !ok {
		return goerr.New("unknown prompt id", goerr.V("prompt_id", string(id)))
	}
	tmpl, err := template.New(string(id)).Option("missingkey=error").Parse(content)
	if err != nil {
		return goerr.Wrap(ErrInvalidTemplate, "parse failed",
			goerr.V("reason", err.Error()))
	}
	for i, probe := range def.probes {
		if err := tmpl.Execute(io.Discard, probe); err != nil {
			return goerr.Wrap(ErrInvalidTemplate, "execute failed",
				goerr.V("reason", err.Error()),
				goerr.V("probe_index", i))
		}
	}
	return nil
}

func executeTemplate(name, content string, input any) (string, error) {
	tmpl, err := template.New(name).Option("missingkey=error").Parse(content)
	if err != nil {
		return "", goerr.Wrap(err, "parse template", goerr.V("name", name))
	}
	var buf strings.Builder
	if err := tmpl.Execute(&buf, input); err != nil {
		return "", goerr.Wrap(err, "execute template", goerr.V("name", name))
	}
	return buf.String(), nil
}

// InvalidTemplateReason extracts a human-readable reason from an
// ErrInvalidTemplate error chain, or returns "" if not present.
func InvalidTemplateReason(err error) string {
	if !errors.Is(err, ErrInvalidTemplate) {
		return ""
	}
	v, ok := goerr.Values(err)["reason"]
	if !ok {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	return fmt.Sprint(v)
}
