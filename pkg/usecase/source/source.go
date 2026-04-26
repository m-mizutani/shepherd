// Package source contains business logic for managing per-workspace Source
// entries (Notion pages/databases today, Slack channels in the future) and
// the Guard used by tools to enforce the resulting access boundary.
package source

import (
	"context"
	"errors"
	"time"

	"github.com/m-mizutani/goerr/v2"
	"github.com/m-mizutani/shepherd/pkg/domain/interfaces"
	"github.com/m-mizutani/shepherd/pkg/domain/model"
	"github.com/m-mizutani/shepherd/pkg/domain/types"
	"github.com/m-mizutani/shepherd/pkg/service/notion"
)

// NotionFetcher is the subset of *notion.Client used here. Declared as an
// interface so tests can substitute a fake without an HTTP server.
type NotionFetcher interface {
	RetrievePage(ctx context.Context, pageID string) (*notion.PageMeta, error)
	RetrieveDatabase(ctx context.Context, dbID string) (*notion.DatabaseMeta, error)
}

// Validation outcomes surfaced as sentinels so the HTTP layer can map them to
// 400/409 with i18n keys.
var (
	ErrInvalidURL          = errors.New("source: invalid notion url")
	ErrTypeMismatch        = errors.New("source: notion object type does not match URL kind")
	ErrNotionForbidden     = errors.New("source: notion integration is not invited to this object")
	ErrNotionNotFound      = errors.New("source: notion object not found or not visible")
	ErrNotionUnauthorized  = errors.New("source: notion token is invalid")
	ErrDuplicate           = errors.New("source: a source with the same provider/object already exists")
)

// UseCase wires the SourceRepository and the Notion verification path.
type UseCase struct {
	repo     interfaces.SourceRepository
	notion   NotionFetcher
	now      func() time.Time
}

// New constructs a UseCase. notionFetcher may be nil when Notion integration
// is disabled — VerifyNotionTarget will then return an error.
func New(repo interfaces.SourceRepository, notionFetcher NotionFetcher, now func() time.Time) *UseCase {
	if now == nil {
		now = time.Now
	}
	return &UseCase{repo: repo, notion: notionFetcher, now: now}
}

// VerifyNotionTarget parses the URL, calls the Notion API to confirm
// reachability, and returns the resolved NotionSource payload (without
// persisting anything).
func (u *UseCase) VerifyNotionTarget(ctx context.Context, raw string) (*model.NotionSource, error) {
	if u.notion == nil {
		return nil, goerr.New("notion integration disabled")
	}
	objType, id, err := notion.ParseURL(raw)
	if err != nil {
		return nil, goerr.Wrap(ErrInvalidURL, err.Error(), goerr.V("input", raw))
	}

	switch objType {
	case types.NotionObjectPage:
		pm, err := u.notion.RetrievePage(ctx, id)
		if err != nil {
			return nil, classifyNotionErr(err)
		}
		return &model.NotionSource{
			ObjectType: types.NotionObjectPage,
			ObjectID:   id,
			URL:        raw,
			Title:      pm.Title,
		}, nil
	case types.NotionObjectDatabase:
		dm, err := u.notion.RetrieveDatabase(ctx, id)
		if err != nil {
			return nil, classifyNotionErr(err)
		}
		return &model.NotionSource{
			ObjectType: types.NotionObjectDatabase,
			ObjectID:   id,
			URL:        raw,
			Title:      dm.Title,
		}, nil
	default:
		return nil, goerr.Wrap(ErrInvalidURL, "unknown notion object type",
			goerr.V("type", string(objType)))
	}
}

// CreateNotionSource verifies and persists a Source. Returns ErrDuplicate when
// the same (provider, objectID) is already registered for this workspace.
func (u *UseCase) CreateNotionSource(ctx context.Context, ws types.WorkspaceID, raw, description, createdBy string) (*model.Source, error) {
	notionSrc, err := u.VerifyNotionTarget(ctx, raw)
	if err != nil {
		return nil, err
	}
	existing, err := u.repo.ListByProvider(ctx, ws, types.SourceProviderNotion)
	if err != nil {
		return nil, goerr.Wrap(err, "failed to scan existing sources for duplicate check")
	}
	for _, s := range existing {
		if s.Notion != nil && s.Notion.ObjectID == notionSrc.ObjectID {
			return nil, goerr.Wrap(ErrDuplicate, "duplicate notion source",
				goerr.V("workspace_id", string(ws)),
				goerr.V("object_id", notionSrc.ObjectID))
		}
	}
	created, err := u.repo.Create(ctx, &model.Source{
		WorkspaceID: ws,
		Provider:    types.SourceProviderNotion,
		Description: description,
		Notion:      notionSrc,
		CreatedAt:   u.now(),
		CreatedBy:   createdBy,
	})
	if err != nil {
		return nil, goerr.Wrap(err, "failed to persist source")
	}
	return created, nil
}

func (u *UseCase) List(ctx context.Context, ws types.WorkspaceID) ([]*model.Source, error) {
	return u.repo.List(ctx, ws)
}

func (u *UseCase) Delete(ctx context.Context, ws types.WorkspaceID, id types.SourceID) error {
	return u.repo.Delete(ctx, ws, id)
}

func classifyNotionErr(err error) error {
	switch {
	case errors.Is(err, notion.ErrUnauthorized):
		return goerr.Wrap(ErrNotionUnauthorized, err.Error())
	case errors.Is(err, notion.ErrForbidden):
		return goerr.Wrap(ErrNotionForbidden, err.Error())
	case errors.Is(err, notion.ErrNotFound):
		return goerr.Wrap(ErrNotionNotFound, err.Error())
	default:
		return goerr.Wrap(err, "notion verification failed")
	}
}
