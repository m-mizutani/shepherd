package http

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/m-mizutani/goerr/v2"
	"github.com/m-mizutani/shepherd/pkg/domain/interfaces"
	"github.com/m-mizutani/shepherd/pkg/domain/model"
	"github.com/m-mizutani/shepherd/pkg/domain/model/config"
	"github.com/m-mizutani/shepherd/pkg/usecase"
	"github.com/m-mizutani/shepherd/pkg/utils/errutil"
)

type APIHandler struct {
	workspaceUC *usecase.WorkspaceUseCase
	ticketUC    *usecase.TicketUseCase
}

var _ ServerInterface = (*APIHandler)(nil)

func NewAPIHandler(registry *model.WorkspaceRegistry, repo interfaces.Repository) *APIHandler {
	return &APIHandler{
		workspaceUC: usecase.NewWorkspaceUseCase(registry),
		ticketUC:    usecase.NewTicketUseCase(repo, registry),
	}
}

func (h *APIHandler) GetHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(r.Context(), w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *APIHandler) ListWorkspaces(w http.ResponseWriter, r *http.Request) {
	workspaces := h.workspaceUC.List()
	resp := make([]Workspace, 0, len(workspaces))
	for _, ws := range workspaces {
		resp = append(resp, Workspace{Id: ws.ID, Name: ws.Name})
	}
	writeJSON(r.Context(), w, http.StatusOK, map[string][]Workspace{"workspaces": resp})
}

func (h *APIHandler) GetWorkspace(w http.ResponseWriter, r *http.Request, workspaceId WorkspaceId) {
	ws, err := h.workspaceUC.Get(string(workspaceId))
	if err != nil {
		handleUseCaseError(r.Context(), w, err)
		return
	}
	writeJSON(r.Context(), w, http.StatusOK, Workspace{Id: ws.ID, Name: ws.Name})
}

func (h *APIHandler) GetWorkspaceConfig(w http.ResponseWriter, r *http.Request, workspaceId WorkspaceId) {
	schema, err := h.workspaceUC.GetConfig(string(workspaceId))
	if err != nil {
		handleUseCaseError(r.Context(), w, err)
		return
	}
	writeJSON(r.Context(), w, http.StatusOK, toWorkspaceConfigResponse(schema))
}

func (h *APIHandler) ListTickets(w http.ResponseWriter, r *http.Request, workspaceId WorkspaceId, params ListTicketsParams) {
	var statusIDs []string
	if params.StatusId != nil {
		statusIDs = []string{*params.StatusId}
	}

	tickets, err := h.ticketUC.List(r.Context(), string(workspaceId), params.IsClosed, statusIDs)
	if err != nil {
		handleUseCaseError(r.Context(), w, err)
		return
	}

	resp := make([]Ticket, 0, len(tickets))
	for _, t := range tickets {
		resp = append(resp, toTicketResponse(t))
	}
	writeJSON(r.Context(), w, http.StatusOK, map[string][]Ticket{"tickets": resp})
}

func (h *APIHandler) CreateTicket(w http.ResponseWriter, r *http.Request, workspaceId WorkspaceId) {
	var req CreateTicketRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		errutil.HandleHTTP(r.Context(), w, goerr.Wrap(err, "invalid request body"), http.StatusBadRequest)
		return
	}

	var statusID, assigneeID string
	if req.StatusId != nil {
		statusID = *req.StatusId
	}
	if req.AssigneeId != nil {
		assigneeID = *req.AssigneeId
	}

	fields := toModelFieldValues(req.Fields)

	var description string
	if req.Description != nil {
		description = *req.Description
	}

	ticket, err := h.ticketUC.Create(r.Context(), string(workspaceId), req.Title, description, statusID, assigneeID, fields)
	if err != nil {
		handleUseCaseError(r.Context(), w, err)
		return
	}

	writeJSON(r.Context(), w, http.StatusCreated, toTicketResponse(ticket))
}

func (h *APIHandler) GetTicket(w http.ResponseWriter, r *http.Request, workspaceId WorkspaceId, ticketId TicketId) {
	ticket, err := h.ticketUC.Get(r.Context(), string(workspaceId), string(ticketId))
	if err != nil {
		handleUseCaseError(r.Context(), w, err)
		return
	}
	writeJSON(r.Context(), w, http.StatusOK, toTicketResponse(ticket))
}

func (h *APIHandler) UpdateTicket(w http.ResponseWriter, r *http.Request, workspaceId WorkspaceId, ticketId TicketId) {
	var req UpdateTicketRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		errutil.HandleHTTP(r.Context(), w, goerr.Wrap(err, "invalid request body"), http.StatusBadRequest)
		return
	}

	fields := toModelFieldValues(req.Fields)

	ticket, err := h.ticketUC.Update(r.Context(), string(workspaceId), string(ticketId), req.Title, req.Description, req.StatusId, req.AssigneeId, fields)
	if err != nil {
		handleUseCaseError(r.Context(), w, err)
		return
	}

	writeJSON(r.Context(), w, http.StatusOK, toTicketResponse(ticket))
}

func (h *APIHandler) DeleteTicket(w http.ResponseWriter, r *http.Request, workspaceId WorkspaceId, ticketId TicketId) {
	if err := h.ticketUC.Delete(r.Context(), string(workspaceId), string(ticketId)); err != nil {
		handleUseCaseError(r.Context(), w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *APIHandler) ListComments(w http.ResponseWriter, r *http.Request, workspaceId WorkspaceId, ticketId TicketId) {
	comments, err := h.ticketUC.ListComments(r.Context(), string(workspaceId), string(ticketId))
	if err != nil {
		handleUseCaseError(r.Context(), w, err)
		return
	}

	resp := make([]Comment, 0, len(comments))
	for _, c := range comments {
		resp = append(resp, Comment{
			Id:          c.ID,
			SlackUserId: c.SlackUserID,
			Body:        c.Body,
			CreatedAt:   c.CreatedAt,
		})
	}
	writeJSON(r.Context(), w, http.StatusOK, map[string][]Comment{"comments": resp})
}

func toTicketResponse(t *model.Ticket) Ticket {
	fields := make([]FieldValue, 0, len(t.FieldValues))
	for _, fv := range t.FieldValues {
		fields = append(fields, FieldValue{
			FieldId: fv.FieldID,
			Value:   fv.Value,
		})
	}

	ticket := Ticket{
		Id:        t.ID,
		SeqNum:    t.SeqNum,
		Title:     t.Title,
		StatusId:  t.StatusID,
		Fields:    fields,
		CreatedAt: t.CreatedAt,
		UpdatedAt: t.UpdatedAt,
	}

	if t.Description != "" {
		ticket.Description = &t.Description
	}
	if t.AssigneeID != "" {
		ticket.AssigneeId = &t.AssigneeID
	}
	if t.ReporterSlackUserID != "" {
		ticket.ReporterSlackUserId = &t.ReporterSlackUserID
	}
	if t.SlackChannelID != "" {
		ticket.SlackChannelId = &t.SlackChannelID
	}
	if t.SlackThreadTS != "" {
		ticket.SlackThreadTs = &t.SlackThreadTS
	}

	return ticket
}

func toWorkspaceConfigResponse(schema *config.FieldSchema) WorkspaceConfig {
	statuses := make([]StatusDef, 0, len(schema.Statuses))
	for _, s := range schema.Statuses {
		statuses = append(statuses, StatusDef{
			Id:       s.ID,
			Name:     s.Name,
			Color:    s.Color,
			Order:    s.Order,
			IsClosed: schema.IsClosedStatus(s.ID),
		})
	}

	fields := make([]FieldDefinition, 0, len(schema.Fields))
	for _, f := range schema.Fields {
		fd := FieldDefinition{
			Id:       f.ID,
			Name:     f.Name,
			Type:     FieldType(f.Type),
			Required: f.Required,
		}
		if f.Description != "" {
			fd.Description = &f.Description
		}
		if len(f.Options) > 0 {
			opts := make([]FieldOption, 0, len(f.Options))
			for _, o := range f.Options {
				fo := FieldOption{Id: o.ID, Name: o.Name}
				if o.Color != "" {
					fo.Color = &o.Color
				}
				if len(o.Metadata) > 0 {
					md := map[string]interface{}{}
					for k, v := range o.Metadata {
						md[k] = v
					}
					fo.Metadata = &md
				}
				opts = append(opts, fo)
			}
			fd.Options = &opts
		}
		fields = append(fields, fd)
	}

	closedIDs := schema.TicketConfig.ClosedStatusIDs
	if closedIDs == nil {
		closedIDs = []string{}
	}

	return WorkspaceConfig{
		Statuses: statuses,
		TicketConfig: TicketConfig{
			DefaultStatusId: schema.TicketConfig.DefaultStatusID,
			ClosedStatusIds: closedIDs,
		},
		Fields: fields,
		Labels: EntityLabels{
			Ticket:      schema.Labels.Ticket,
			Title:       schema.Labels.Title,
			Description: schema.Labels.Description,
		},
	}
}

func toModelFieldValues(fields *[]FieldValue) map[string]model.FieldValue {
	if fields == nil {
		return nil
	}
	result := make(map[string]model.FieldValue, len(*fields))
	for _, f := range *fields {
		result[f.FieldId] = model.FieldValue{
			FieldID: f.FieldId,
			Value:   f.Value,
		}
	}
	return result
}

func handleUseCaseError(ctx context.Context, w http.ResponseWriter, err error) {
	if goerr.HasTag(err, errutil.TagNotFound) {
		errutil.HandleHTTP(ctx, w, err, http.StatusNotFound)
		return
	}
	errutil.HandleHTTP(ctx, w, err, http.StatusInternalServerError)
}
