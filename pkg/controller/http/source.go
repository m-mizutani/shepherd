package http

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/m-mizutani/goerr/v2"
	"github.com/m-mizutani/shepherd/pkg/domain/model"
	"github.com/m-mizutani/shepherd/pkg/domain/model/auth"
	"github.com/m-mizutani/shepherd/pkg/domain/types"
	"github.com/m-mizutani/shepherd/pkg/usecase/source"
	"github.com/m-mizutani/shepherd/pkg/utils/errutil"
)

func (h *APIHandler) ListSources(w http.ResponseWriter, r *http.Request, workspaceId WorkspaceId) {
	if h.sourceUC == nil {
		errutil.HandleHTTP(r.Context(), w, goerr.New("source feature not configured"), http.StatusServiceUnavailable)
		return
	}
	srcs, err := h.sourceUC.List(r.Context(), types.WorkspaceID(workspaceId))
	if err != nil {
		handleUseCaseError(r.Context(), w, err)
		return
	}
	out := make([]Source, 0, len(srcs))
	for _, s := range srcs {
		out = append(out, toSourceResponse(s))
	}
	writeJSON(r.Context(), w, http.StatusOK, map[string][]Source{"sources": out})
}

func (h *APIHandler) CreateSource(w http.ResponseWriter, r *http.Request, workspaceId WorkspaceId) {
	if h.sourceUC == nil {
		errutil.HandleHTTP(r.Context(), w, goerr.New("source feature not configured"), http.StatusServiceUnavailable)
		return
	}
	var req CreateSourceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		errutil.HandleHTTP(r.Context(), w, goerr.Wrap(err, "invalid request body"), http.StatusBadRequest)
		return
	}
	if req.Provider != CreateSourceRequestProviderNotion {
		errutil.HandleHTTP(r.Context(), w, goerr.New("unsupported provider"), http.StatusBadRequest)
		return
	}
	createdBy := tokenSubFromCtx(r.Context())
	description := ""
	if req.Description != nil {
		description = *req.Description
	}
	s, err := h.sourceUC.CreateNotionSource(r.Context(), types.WorkspaceID(workspaceId), req.Url, description, createdBy)
	if err != nil {
		writeSourceError(r.Context(), w, err)
		return
	}
	writeJSON(r.Context(), w, http.StatusCreated, toSourceResponse(s))
}

func (h *APIHandler) DeleteSource(w http.ResponseWriter, r *http.Request, workspaceId WorkspaceId, sourceId string) {
	if h.sourceUC == nil {
		errutil.HandleHTTP(r.Context(), w, goerr.New("source feature not configured"), http.StatusServiceUnavailable)
		return
	}
	if err := h.sourceUC.Delete(r.Context(), types.WorkspaceID(workspaceId), types.SourceID(sourceId)); err != nil {
		handleUseCaseError(r.Context(), w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *APIHandler) ListToolSettings(w http.ResponseWriter, r *http.Request, workspaceId WorkspaceId) {
	if h.catalog == nil {
		errutil.HandleHTTP(r.Context(), w, goerr.New("tool catalog not configured"), http.StatusServiceUnavailable)
		return
	}
	states, err := h.catalog.States(r.Context(), types.WorkspaceID(workspaceId))
	if err != nil {
		handleUseCaseError(r.Context(), w, err)
		return
	}
	out := make([]ToolState, 0, len(states))
	for _, st := range states {
		ts := ToolState{
			ProviderId:     string(st.ID),
			Available:      st.Available,
			DefaultEnabled: st.DefaultEnabled,
			Enabled:        st.Enabled,
		}
		if st.Reason != "" {
			reason := ToolStateReason(st.Reason)
			ts.Reason = &reason
		}
		out = append(out, ts)
	}
	writeJSON(r.Context(), w, http.StatusOK, map[string][]ToolState{"tools": out})
}

func (h *APIHandler) SetToolEnabled(w http.ResponseWriter, r *http.Request, workspaceId WorkspaceId, providerId string) {
	if h.repo == nil {
		errutil.HandleHTTP(r.Context(), w, goerr.New("repository not configured"), http.StatusInternalServerError)
		return
	}
	var body SetToolEnabledJSONBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		errutil.HandleHTTP(r.Context(), w, goerr.Wrap(err, "invalid request body"), http.StatusBadRequest)
		return
	}
	if err := h.repo.ToolSettings().Set(r.Context(), types.WorkspaceID(workspaceId), providerId, body.Enabled); err != nil {
		handleUseCaseError(r.Context(), w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func toSourceResponse(s *model.Source) Source {
	out := Source{
		Id:          string(s.ID),
		WorkspaceId: string(s.WorkspaceID),
		Provider:    SourceProvider(string(s.Provider)),
		CreatedAt:   s.CreatedAt,
	}
	if s.CreatedBy != "" {
		v := s.CreatedBy
		out.CreatedBy = &v
	}
	if s.Description != "" {
		v := s.Description
		out.Description = &v
	}
	if s.Notion != nil {
		ns := NotionSource{
			ObjectType: NotionSourceObjectType(string(s.Notion.ObjectType)),
			ObjectId:   s.Notion.ObjectID,
			Url:        s.Notion.URL,
		}
		if s.Notion.Title != "" {
			t := s.Notion.Title
			ns.Title = &t
		}
		out.Notion = &ns
	}
	return out
}

func tokenSubFromCtx(ctx context.Context) string {
	tok, err := auth.TokenFromContext(ctx)
	if err != nil || tok == nil {
		return ""
	}
	return tok.Sub
}

func writeSourceError(ctx context.Context, w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, source.ErrInvalidURL),
		errors.Is(err, source.ErrTypeMismatch):
		errutil.HandleHTTP(ctx, w, err, http.StatusBadRequest)
	case errors.Is(err, source.ErrNotionForbidden):
		// Distinct status so the WebUI can render the "invite the integration"
		// message without parsing error strings.
		errutil.HandleHTTP(ctx, w, err, http.StatusForbidden)
	case errors.Is(err, source.ErrNotionNotFound):
		errutil.HandleHTTP(ctx, w, err, http.StatusNotFound)
	case errors.Is(err, source.ErrDuplicate):
		errutil.HandleHTTP(ctx, w, err, http.StatusConflict)
	case errors.Is(err, source.ErrNotionUnauthorized):
		errutil.HandleHTTP(ctx, w, err, http.StatusInternalServerError)
	default:
		errutil.HandleHTTP(ctx, w, err, http.StatusInternalServerError)
	}
}
