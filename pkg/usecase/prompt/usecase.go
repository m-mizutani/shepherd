package prompt

import (
	"context"
	"log/slog"
	"strings"
	"time"

	"github.com/m-mizutani/goerr/v2"
	"github.com/m-mizutani/shepherd/pkg/domain/interfaces"
	"github.com/m-mizutani/shepherd/pkg/domain/model"
	"github.com/m-mizutani/shepherd/pkg/domain/types"
	"github.com/m-mizutani/shepherd/pkg/utils/errutil"
	"github.com/m-mizutani/shepherd/pkg/utils/logging"
)

// UseCase manages workspace-level prompt overrides. It is also the single
// place that renders prompts for downstream agents (e.g. triage planner),
// so override lookup and rendering are colocated.
//
// The user-facing override stored here is workspace-specific *additional
// guidance* — opaque markdown that gets embedded into the shepherd-managed
// base template at render time. The base template itself is not editable.
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

// slotDefs enumerates every PromptID this usecase recognises. Membership is
// the only fact callers depend on (lookups for unknown IDs error out), so the
// value is a zero-sized struct.
var slotDefs = map[model.PromptID]struct{}{
	model.PromptIDTriage: {},
}

// Effective returns the user-supplied additional guidance for (ws, id) along
// with its version number. version == 0 means no override exists yet and the
// returned content is empty (the slot is using the bare base prompt).
func (u *UseCase) Effective(ctx context.Context, ws types.WorkspaceID, id model.PromptID) (string, int, error) {
	if _, ok := slotDefs[id]; !ok {
		return "", 0, goerr.New("unknown prompt id", goerr.V("prompt_id", string(id)))
	}
	cur, err := u.Current(ctx, ws, id)
	if err != nil {
		return "", 0, err
	}
	if cur == nil {
		return "", 0, nil
	}
	return cur.Content, cur.Version, nil
}

// Current returns the latest stored override for (ws, id), or (nil, nil)
// when no override has been saved yet. This is a single point read against
// the repository's GetCurrent — controllers needing the latest version's
// metadata (updatedAt, updatedBy, length) should call this instead of
// fetching the full History list.
func (u *UseCase) Current(ctx context.Context, ws types.WorkspaceID, id model.PromptID) (*model.PromptVersion, error) {
	if _, ok := slotDefs[id]; !ok {
		return nil, goerr.New("unknown prompt id", goerr.V("prompt_id", string(id)))
	}
	if u.repo == nil {
		return nil, nil
	}
	cur, err := u.repo.GetCurrent(ctx, ws, id)
	if err != nil {
		return nil, goerr.Wrap(err, "load current prompt override",
			goerr.V("workspace_id", string(ws)),
			goerr.V("prompt_id", string(id)))
	}
	return cur, nil
}

// Save atomically appends content as the next version. Content is stored
// verbatim — it is not parsed as a Go template — so the only validation here
// is that the slot is known and the optimistic-lock version is fresh.
// version must equal current+1 (or 1 when no override exists yet); otherwise
// the call is rejected with ErrPromptVersionConflict.
func (u *UseCase) Save(ctx context.Context, ws types.WorkspaceID, id model.PromptID, version int, content string, by Author) (*model.PromptVersion, error) {
	if u.repo == nil {
		return nil, goerr.New("prompt repository not configured")
	}
	if _, ok := slotDefs[id]; !ok {
		return nil, goerr.New("unknown prompt id", goerr.V("prompt_id", string(id)))
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

// RenderTriagePlan looks up the workspace-specific additional guidance for ws
// and embeds it into the shepherd-managed base template. Only the base
// template is executed through text/template; the user content is treated as
// opaque markdown. Repository failures log and continue with an empty
// guidance string, so a flapping data layer does not take triage down.
func (u *UseCase) RenderTriagePlan(ctx context.Context, ws types.WorkspaceID, in TriagePlanInput) (string, error) {
	guidance, _, err := u.Effective(ctx, ws, model.PromptIDTriage)
	if err != nil {
		errutil.Handle(ctx,
			goerr.Wrap(err, "load triage prompt guidance failed; continuing without",
				goerr.V("workspace_id", string(ws))))
		logging.From(ctx).Warn("triage prompt guidance lookup failed; continuing without",
			slog.String("workspace_id", string(ws)))
		guidance = ""
	}
	in.UserGuidance = strings.TrimSpace(guidance)
	return RenderTriagePlan(in)
}
