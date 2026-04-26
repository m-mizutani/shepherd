package firestore

import (
	"context"
	"time"

	"cloud.google.com/go/firestore"
	"github.com/m-mizutani/goerr/v2"
	"github.com/m-mizutani/shepherd/pkg/domain/model"
	"github.com/m-mizutani/shepherd/pkg/domain/types"
)

type toolSettingsRepository struct {
	client *firestore.Client
}

func (r *toolSettingsRepository) ref(ws types.WorkspaceID) *firestore.DocumentRef {
	return r.client.Collection("workspaces").Doc(string(ws)).Collection("tool_settings").Doc("current")
}

func (r *toolSettingsRepository) Get(ctx context.Context, ws types.WorkspaceID) (*model.ToolSettings, error) {
	doc, err := r.ref(ws).Get(ctx)
	if err != nil {
		if isNotFound(err) {
			return &model.ToolSettings{
				WorkspaceID: ws,
				Enabled:     map[string]bool{},
			}, nil
		}
		return nil, goerr.Wrap(err, "failed to get tool_settings")
	}
	var s model.ToolSettings
	if err := doc.DataTo(&s); err != nil {
		return nil, goerr.Wrap(err, "failed to decode tool_settings")
	}
	s.WorkspaceID = ws
	if s.Enabled == nil {
		s.Enabled = map[string]bool{}
	}
	return &s, nil
}

func (r *toolSettingsRepository) Set(ctx context.Context, ws types.WorkspaceID, providerID string, enabled bool) error {
	// Update only the toggled key + UpdatedAt via field-path merge so two
	// concurrent toggles for different providers do not clobber each other.
	// Set with firestore.Merge handles both create-if-absent and atomic
	// per-field write — RunTransaction would also work but adds round-trips.
	payload := map[string]any{
		"WorkspaceID": string(ws),
		"Enabled":     map[string]bool{providerID: enabled},
		"UpdatedAt":   time.Now(),
	}
	if _, err := r.ref(ws).Set(ctx, payload, firestore.Merge(
		[]string{"WorkspaceID"},
		[]string{"Enabled", providerID},
		[]string{"UpdatedAt"},
	)); err != nil {
		return goerr.Wrap(err, "failed to set tool_settings")
	}
	return nil
}
