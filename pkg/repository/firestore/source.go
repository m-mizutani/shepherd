package firestore

import (
	"context"
	"sort"

	"cloud.google.com/go/firestore"
	"github.com/google/uuid"
	"github.com/m-mizutani/goerr/v2"
	"github.com/m-mizutani/shepherd/pkg/domain/model"
	"github.com/m-mizutani/shepherd/pkg/domain/types"
	"google.golang.org/api/iterator"
)

type sourceRepository struct {
	client *firestore.Client
}

func (r *sourceRepository) collection(ws types.WorkspaceID) *firestore.CollectionRef {
	return r.client.Collection("workspaces").Doc(string(ws)).Collection("sources")
}

func (r *sourceRepository) Create(ctx context.Context, s *model.Source) (*model.Source, error) {
	if s == nil {
		return nil, goerr.New("source is nil")
	}
	if s.WorkspaceID == "" {
		return nil, goerr.New("source workspace_id is required")
	}
	if s.ID == "" {
		s.ID = types.SourceID(uuid.Must(uuid.NewV7()).String())
	}
	ref := r.collection(s.WorkspaceID).Doc(string(s.ID))
	if _, err := ref.Set(ctx, s); err != nil {
		return nil, goerr.Wrap(err, "failed to create source")
	}
	return s, nil
}

func (r *sourceRepository) Get(ctx context.Context, ws types.WorkspaceID, id types.SourceID) (*model.Source, error) {
	doc, err := r.collection(ws).Doc(string(id)).Get(ctx)
	if err != nil {
		if isNotFound(err) {
			return nil, goerr.New("source not found",
				goerr.V("workspace_id", string(ws)),
				goerr.V("source_id", string(id)))
		}
		return nil, goerr.Wrap(err, "failed to get source")
	}
	var s model.Source
	if err := doc.DataTo(&s); err != nil {
		return nil, goerr.Wrap(err, "failed to decode source")
	}
	s.ID = types.SourceID(doc.Ref.ID)
	s.WorkspaceID = ws
	return &s, nil
}

func (r *sourceRepository) List(ctx context.Context, ws types.WorkspaceID) ([]*model.Source, error) {
	// Per the project rule (CLAUDE.md "Firestore Storage"): no server-side
	// filter+order combinations that require composite indexes. Fetch the
	// per-workspace collection unsorted, then sort in Go. Source counts per
	// workspace are bounded (handful of pages/databases), so this is cheap.
	out, err := r.iterate(ctx, ws, r.collection(ws).Documents(ctx))
	if err != nil {
		return nil, err
	}
	sortByCreatedAtAsc(out)
	return out, nil
}

func (r *sourceRepository) ListByProvider(ctx context.Context, ws types.WorkspaceID, p types.SourceProvider) ([]*model.Source, error) {
	all, err := r.List(ctx, ws)
	if err != nil {
		return nil, err
	}
	out := make([]*model.Source, 0, len(all))
	for _, s := range all {
		if s.Provider == p {
			out = append(out, s)
		}
	}
	return out, nil
}

func sortByCreatedAtAsc(srcs []*model.Source) {
	sort.SliceStable(srcs, func(i, j int) bool {
		return srcs[i].CreatedAt.Before(srcs[j].CreatedAt)
	})
}

func (r *sourceRepository) Delete(ctx context.Context, ws types.WorkspaceID, id types.SourceID) error {
	ref := r.collection(ws).Doc(string(id))
	if _, err := ref.Get(ctx); err != nil {
		if isNotFound(err) {
			return goerr.New("source not found",
				goerr.V("workspace_id", string(ws)),
				goerr.V("source_id", string(id)))
		}
		return goerr.Wrap(err, "failed to lookup source for delete")
	}
	if _, err := ref.Delete(ctx); err != nil {
		return goerr.Wrap(err, "failed to delete source")
	}
	return nil
}

func (r *sourceRepository) iterate(_ context.Context, ws types.WorkspaceID, iter *firestore.DocumentIterator) ([]*model.Source, error) {
	defer iter.Stop()
	var out []*model.Source
	for {
		doc, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, goerr.Wrap(err, "failed to iterate sources")
		}
		var s model.Source
		if err := doc.DataTo(&s); err != nil {
			return nil, goerr.Wrap(err, "failed to decode source")
		}
		s.ID = types.SourceID(doc.Ref.ID)
		s.WorkspaceID = ws
		out = append(out, &s)
	}
	return out, nil
}
