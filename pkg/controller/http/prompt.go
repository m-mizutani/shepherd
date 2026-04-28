package http

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/m-mizutani/goerr/v2"
	"github.com/m-mizutani/shepherd/pkg/domain/interfaces"
	"github.com/m-mizutani/shepherd/pkg/domain/model"
	"github.com/m-mizutani/shepherd/pkg/domain/model/auth"
	"github.com/m-mizutani/shepherd/pkg/domain/types"
	"github.com/m-mizutani/shepherd/pkg/usecase/prompt"
	"github.com/m-mizutani/shepherd/pkg/utils/errutil"
)

// promptCatalog is the static list of slots the UI advertises. customizable
// flips to true once the slot is wired through the usecase / triage path. New
// slots are added here first; the corresponding PromptID + slotDef in the
// usecase layer can lag behind without breaking the API.
var promptCatalog = []promptSlotMeta{
	{
		ID:           string(model.PromptIDTriage),
		Label:        "Triage",
		Description:  "Classify, prioritize, and route incoming tickets.",
		Customizable: true,
	},
}

type promptSlotMeta struct {
	ID           string
	Label        string
	Description  string
	Customizable bool
}

// promptVariables is the fixed list of template variables the UI surfaces in
// the editor footer. Keep in sync with prompt.TriagePlanInput field names.
var promptVariables = []string{
	"Title",
	"Description",
	"InitialMessage",
	"Reporter",
}

func (h *APIHandler) ListPrompts(w http.ResponseWriter, r *http.Request, workspaceId WorkspaceId) {
	if h.promptUC == nil {
		errutil.HandleHTTP(r.Context(), w, goerr.New("prompt usecase not configured"), http.StatusServiceUnavailable)
		return
	}
	ws := types.WorkspaceID(workspaceId)
	out := make([]PromptSlot, 0, len(promptCatalog))
	for _, m := range promptCatalog {
		slot := PromptSlot{
			Id:           m.ID,
			Label:        m.Label,
			Description:  m.Description,
			Customizable: m.Customizable,
		}
		if m.Customizable {
			cur, err := h.promptUC.Current(r.Context(), ws, model.PromptID(m.ID))
			if err != nil {
				handleUseCaseError(r.Context(), w, err)
				return
			}
			if cur != nil {
				slot.Length = len(cur.Content)
				slot.Version = cur.Version
				slot.Configured = true
				at := cur.UpdatedAt
				slot.UpdatedAt = &at
				if cur.UpdatedBy != "" {
					slot.UpdatedBy = toPromptAuthor(cur)
				}
			} else {
				def, err := h.promptUC.Default(model.PromptID(m.ID))
				if err != nil {
					handleUseCaseError(r.Context(), w, err)
					return
				}
				slot.Length = len(def)
			}
		}
		out = append(out, slot)
	}
	writeJSON(r.Context(), w, http.StatusOK, map[string][]PromptSlot{"prompts": out})
}

func (h *APIHandler) GetPrompt(w http.ResponseWriter, r *http.Request, workspaceId WorkspaceId, promptId PromptId) {
	if h.promptUC == nil {
		errutil.HandleHTTP(r.Context(), w, goerr.New("prompt usecase not configured"), http.StatusServiceUnavailable)
		return
	}
	id, ok := lookupPromptID(promptId)
	if !ok {
		http.NotFound(w, r)
		return
	}
	ws := types.WorkspaceID(workspaceId)
	cur, err := h.promptUC.Current(r.Context(), ws, id)
	if err != nil {
		handleUseCaseError(r.Context(), w, err)
		return
	}
	def, err := h.promptUC.Default(id)
	if err != nil {
		handleUseCaseError(r.Context(), w, err)
		return
	}
	resp := PromptDetail{
		Id:             string(id),
		DefaultContent: def,
		Variables:      promptVariables,
	}
	if cur != nil {
		resp.Content = cur.Content
		resp.Version = cur.Version
		resp.IsOverride = true
		at := cur.UpdatedAt
		resp.UpdatedAt = &at
		if cur.UpdatedBy != "" {
			resp.UpdatedBy = toPromptAuthor(cur)
		}
	} else {
		resp.Content = def
		resp.Version = 0
		resp.IsOverride = false
	}
	writeJSON(r.Context(), w, http.StatusOK, resp)
}

func (h *APIHandler) SavePrompt(w http.ResponseWriter, r *http.Request, workspaceId WorkspaceId, promptId PromptId) {
	if h.promptUC == nil {
		errutil.HandleHTTP(r.Context(), w, goerr.New("prompt usecase not configured"), http.StatusServiceUnavailable)
		return
	}
	id, ok := lookupPromptID(promptId)
	if !ok {
		http.NotFound(w, r)
		return
	}
	var body SavePromptJSONBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		errutil.HandleHTTP(r.Context(), w, goerr.Wrap(err, "invalid request body"), http.StatusBadRequest)
		return
	}
	if body.Version < 1 {
		errutil.HandleHTTP(r.Context(), w, goerr.New("version must be >= 1"), http.StatusBadRequest)
		return
	}
	author, err := authorFromContext(r.Context())
	if err != nil {
		errutil.HandleHTTP(r.Context(), w, err, http.StatusUnauthorized)
		return
	}
	saved, err := h.promptUC.Save(r.Context(), types.WorkspaceID(workspaceId), id,
		body.Version, body.Content, author)
	if err != nil {
		writePromptError(r.Context(), w, err, types.WorkspaceID(workspaceId), id, h.promptUC)
		return
	}
	writeJSON(r.Context(), w, http.StatusOK, toPromptVersionResponse(saved, true))
}

func (h *APIHandler) ListPromptHistory(w http.ResponseWriter, r *http.Request, workspaceId WorkspaceId, promptId PromptId) {
	if h.promptUC == nil {
		errutil.HandleHTTP(r.Context(), w, goerr.New("prompt usecase not configured"), http.StatusServiceUnavailable)
		return
	}
	id, ok := lookupPromptID(promptId)
	if !ok {
		http.NotFound(w, r)
		return
	}
	hist, err := h.promptUC.History(r.Context(), types.WorkspaceID(workspaceId), id)
	if err != nil {
		handleUseCaseError(r.Context(), w, err)
		return
	}
	out := make([]PromptVersion, 0, len(hist))
	for i, v := range hist {
		out = append(out, toPromptVersionResponse(v, i == len(hist)-1))
	}
	writeJSON(r.Context(), w, http.StatusOK, map[string][]PromptVersion{"versions": out})
}

func (h *APIHandler) RestorePrompt(w http.ResponseWriter, r *http.Request, workspaceId WorkspaceId, promptId PromptId) {
	if h.promptUC == nil {
		errutil.HandleHTTP(r.Context(), w, goerr.New("prompt usecase not configured"), http.StatusServiceUnavailable)
		return
	}
	id, ok := lookupPromptID(promptId)
	if !ok {
		http.NotFound(w, r)
		return
	}
	var body RestorePromptJSONBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		errutil.HandleHTTP(r.Context(), w, goerr.Wrap(err, "invalid request body"), http.StatusBadRequest)
		return
	}
	if body.Version < 1 || body.TargetVersion < 1 {
		errutil.HandleHTTP(r.Context(), w, goerr.New("version >= 1 and targetVersion >= 1 are required"), http.StatusBadRequest)
		return
	}
	author, err := authorFromContext(r.Context())
	if err != nil {
		errutil.HandleHTTP(r.Context(), w, err, http.StatusUnauthorized)
		return
	}
	saved, err := h.promptUC.Restore(r.Context(), types.WorkspaceID(workspaceId), id,
		body.Version, body.TargetVersion, author)
	if err != nil {
		writePromptError(r.Context(), w, err, types.WorkspaceID(workspaceId), id, h.promptUC)
		return
	}
	writeJSON(r.Context(), w, http.StatusOK, toPromptVersionResponse(saved, true))
}

func authorFromContext(ctx context.Context) (prompt.Author, error) {
	tok, err := auth.TokenFromContext(ctx)
	if err != nil {
		return prompt.Author{}, goerr.Wrap(err, "missing auth token")
	}
	return prompt.Author{
		Name:  tok.Name,
		Email: tok.Email,
		Sub:   tok.Sub,
	}, nil
}

func lookupPromptID(raw string) (model.PromptID, bool) {
	for _, m := range promptCatalog {
		if m.ID == raw && m.Customizable {
			return model.PromptID(m.ID), true
		}
	}
	return "", false
}

func toPromptAuthor(v *model.PromptVersion) *PromptAuthor {
	a := &PromptAuthor{Name: v.UpdatedBy}
	if v.UpdatedByEmail != "" {
		email := v.UpdatedByEmail
		a.Email = &email
	}
	return a
}

func toPromptVersionResponse(v *model.PromptVersion, current bool) PromptVersion {
	out := PromptVersion{
		Version:   v.Version,
		Content:   v.Content,
		UpdatedAt: v.UpdatedAt,
		Current:   current,
	}
	if v.UpdatedBy != "" {
		out.UpdatedBy = toPromptAuthor(v)
	}
	return out
}

// writePromptError maps usecase / repository sentinels to HTTP responses.
func writePromptError(ctx context.Context, w http.ResponseWriter, err error,
	ws types.WorkspaceID, id model.PromptID, uc *prompt.UseCase) {
	switch {
	case errors.Is(err, prompt.ErrInvalidTemplate):
		body := PromptTemplateError{
			Error:  InvalidTemplate,
			Reason: prompt.InvalidTemplateReason(err),
		}
		writeJSON(ctx, w, http.StatusUnprocessableEntity, body)
	case errors.Is(err, interfaces.ErrPromptVersionConflict):
		current := 0
		if uc != nil {
			if _, v, e := uc.Effective(ctx, ws, id); e == nil {
				current = v
			}
		}
		body := PromptVersionConflict{
			Error:          VersionConflict,
			CurrentVersion: current,
		}
		writeJSON(ctx, w, http.StatusConflict, body)
	case errors.Is(err, interfaces.ErrPromptVersionNotFound):
		errutil.HandleHTTP(ctx, w, err, http.StatusNotFound)
	default:
		handleUseCaseError(ctx, w, err)
	}
}
