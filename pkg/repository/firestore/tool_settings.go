package firestore

import (
	"context"
	"time"

	"cloud.google.com/go/firestore"
	"github.com/m-mizutani/goerr/v2"
	"github.com/m-mizutani/shepherd/pkg/domain/model"
	"github.com/m-mizutani/shepherd/pkg/domain/types"
	"google.golang.org/api/iterator"
)

// Layout:
//
//	workspaces/{wsID}/settings/tools/providers/{providerID}
//	    Enabled    bool
//	    UpdatedAt  time.Time
//
// One document per provider keeps writes naturally atomic (no read-modify-
// write, no field-path Merge) and lets future settings sections live as
// sibling documents under workspaces/{ws}/settings/<section>.
type toolSettingsRepository struct {
	client *firestore.Client
}

type providerDoc struct {
	Enabled   bool
	UpdatedAt time.Time
}

func (r *toolSettingsRepository) providersCollection(ws types.WorkspaceID) *firestore.CollectionRef {
	return r.client.Collection("workspaces").Doc(string(ws)).
		Collection("settings").Doc("tools").
		Collection("providers")
}

func (r *toolSettingsRepository) Get(ctx context.Context, ws types.WorkspaceID) (*model.ToolSettings, error) {
	out := &model.ToolSettings{
		WorkspaceID: ws,
		Enabled:     map[string]bool{},
	}
	iter := r.providersCollection(ws).Documents(ctx)
	defer iter.Stop()
	for {
		doc, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, goerr.Wrap(err, "failed to iterate tool_settings providers",
				goerr.V("workspace_id", string(ws)))
		}
		var pd providerDoc
		if err := doc.DataTo(&pd); err != nil {
			return nil, goerr.Wrap(err, "failed to decode tool_settings provider",
				goerr.V("workspace_id", string(ws)),
				goerr.V("provider_id", doc.Ref.ID))
		}
		out.Enabled[doc.Ref.ID] = pd.Enabled
		if pd.UpdatedAt.After(out.UpdatedAt) {
			out.UpdatedAt = pd.UpdatedAt
		}
	}
	return out, nil
}

func (r *toolSettingsRepository) Set(ctx context.Context, ws types.WorkspaceID, providerID string, enabled bool) error {
	ref := r.providersCollection(ws).Doc(providerID)
	if _, err := ref.Set(ctx, providerDoc{Enabled: enabled, UpdatedAt: time.Now()}); err != nil {
		return goerr.Wrap(err, "failed to set tool_settings",
			goerr.V("workspace_id", string(ws)),
			goerr.V("provider_id", providerID))
	}
	return nil
}
