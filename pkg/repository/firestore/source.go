package firestore

import (
	"context"

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
	return r.iterate(ctx, ws, r.collection(ws).OrderBy("CreatedAt", firestore.Asc).Documents(ctx))
}

func (r *sourceRepository) ListByProvider(ctx context.Context, ws types.WorkspaceID, p types.SourceProvider) ([]*model.Source, error) {
	q := r.collection(ws).Where("Provider", "==", string(p)).OrderBy("CreatedAt", firestore.Asc).Documents(ctx)
	return r.iterate(ctx, ws, q)
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
