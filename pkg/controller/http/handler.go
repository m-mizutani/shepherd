package http

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/m-mizutani/goerr/v2"
	"github.com/m-mizutani/shepherd/pkg/domain/interfaces"
	"github.com/m-mizutani/shepherd/pkg/domain/model"
	"github.com/m-mizutani/shepherd/pkg/domain/model/config"
	"github.com/m-mizutani/shepherd/pkg/domain/types"
	"github.com/m-mizutani/shepherd/pkg/tool"
	"github.com/m-mizutani/shepherd/pkg/usecase"
	"github.com/m-mizutani/shepherd/pkg/usecase/prompt"
	"github.com/m-mizutani/shepherd/pkg/usecase/source"
	"github.com/m-mizutani/shepherd/pkg/utils/errutil"
)

type APIHandler struct {
	workspaceUC *usecase.WorkspaceUseCase
	ticketUC    *usecase.TicketUseCase
	slackUC     *usecase.SlackUseCase
	sourceUC    *source.UseCase
	promptUC    *prompt.UseCase
	repo        interfaces.Repository
	catalog     *tool.Catalog
}

var _ ServerInterface = (*APIHandler)(nil)

func NewAPIHandler(registry *model.WorkspaceRegistry, repo interfaces.Repository, notifier usecase.StatusChangeNotifier, slackUC *usecase.SlackUseCase, sourceUC *source.UseCase, catalog *tool.Catalog, promptUC *prompt.UseCase) *APIHandler {
	return &APIHandler{
		workspaceUC: usecase.NewWorkspaceUseCase(registry),
		ticketUC:    usecase.NewTicketUseCase(repo, registry, notifier),
		slackUC:     slackUC,
		sourceUC:    sourceUC,
		promptUC:    promptUC,
		repo:        repo,
		catalog:     catalog,
	}
}

func (h *APIHandler) GetHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(r.Context(), w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *APIHandler) ListWorkspaces(w http.ResponseWriter, r *http.Request) {
	workspaces := h.workspaceUC.List()
	resp := make([]Workspace, 0, len(workspaces))
	for _, ws := range workspaces {
		resp = append(resp, Workspace{Id: string(ws.ID), Name: ws.Name})
	}
	writeJSON(r.Context(), w, http.StatusOK, map[string][]Workspace{"workspaces": resp})
}

func (h *APIHandler) GetWorkspace(w http.ResponseWriter, r *http.Request, workspaceId WorkspaceId) {
	ws, err := h.workspaceUC.Get(types.WorkspaceID(workspaceId))
	if err != nil {
		handleUseCaseError(r.Context(), w, err)
		return
	}
	writeJSON(r.Context(), w, http.StatusOK, Workspace{Id: string(ws.ID), Name: ws.Name})
}

func (h *APIHandler) GetWorkspaceConfig(w http.ResponseWriter, r *http.Request, workspaceId WorkspaceId) {
	schema, err := h.workspaceUC.GetConfig(types.WorkspaceID(workspaceId))
	if err != nil {
		handleUseCaseError(r.Context(), w, err)
		return
	}
	writeJSON(r.Context(), w, http.StatusOK, toWorkspaceConfigResponse(schema))
}

func (h *APIHandler) ListTickets(w http.ResponseWriter, r *http.Request, workspaceId WorkspaceId, params ListTicketsParams) {
	var statusIDs []types.StatusID
	if params.StatusId != nil {
		statusIDs = []types.StatusID{types.StatusID(*params.StatusId)}
	}

	tickets, err := h.ticketUC.List(r.Context(), types.WorkspaceID(workspaceId), params.IsClosed, statusIDs)
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

	var statusID types.StatusID
	var assigneeIDs []types.SlackUserID
	if req.StatusId != nil {
		statusID = types.StatusID(*req.StatusId)
	}
	if req.AssigneeIds != nil {
		assigneeIDs = make([]types.SlackUserID, 0, len(*req.AssigneeIds))
		for _, id := range *req.AssigneeIds {
			if id == "" {
				continue
			}
			assigneeIDs = append(assigneeIDs, types.SlackUserID(id))
		}
	}

	fields := toModelFieldValues(req.Fields)

	var description string
	if req.Description != nil {
		description = *req.Description
	}

	ticket, err := h.ticketUC.Create(r.Context(), types.WorkspaceID(workspaceId), req.Title, description, statusID, assigneeIDs, fields)
	if err != nil {
		handleUseCaseError(r.Context(), w, err)
		return
	}

	writeJSON(r.Context(), w, http.StatusCreated, toTicketResponse(ticket))
}

func (h *APIHandler) GetTicket(w http.ResponseWriter, r *http.Request, workspaceId WorkspaceId, ticketId TicketId) {
	ticket, err := h.ticketUC.Get(r.Context(), types.WorkspaceID(workspaceId), types.TicketID(ticketId))
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

	var statusID *types.StatusID
	var assigneeIDs *[]types.SlackUserID
	if req.StatusId != nil {
		sid := types.StatusID(*req.StatusId)
		statusID = &sid
	}
	if req.AssigneeIds != nil {
		ids := make([]types.SlackUserID, 0, len(*req.AssigneeIds))
		for _, id := range *req.AssigneeIds {
			if id == "" {
				continue
			}
			ids = append(ids, types.SlackUserID(id))
		}
		assigneeIDs = &ids
	}

	ticket, err := h.ticketUC.Update(r.Context(), types.WorkspaceID(workspaceId), types.TicketID(ticketId), req.Title, req.Description, statusID, assigneeIDs, fields)
	if err != nil {
		handleUseCaseError(r.Context(), w, err)
		return
	}

	writeJSON(r.Context(), w, http.StatusOK, toTicketResponse(ticket))
}

func (h *APIHandler) DeleteTicket(w http.ResponseWriter, r *http.Request, workspaceId WorkspaceId, ticketId TicketId) {
	if err := h.ticketUC.Delete(r.Context(), types.WorkspaceID(workspaceId), types.TicketID(ticketId)); err != nil {
		handleUseCaseError(r.Context(), w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *APIHandler) ListSlackUsers(w http.ResponseWriter, r *http.Request, workspaceId WorkspaceId) {
	if h.slackUC == nil {
		errutil.HandleHTTP(r.Context(), w, goerr.New("slack integration not configured"), http.StatusServiceUnavailable)
		return
	}

	users, err := h.slackUC.ListUsers(r.Context())
	if err != nil {
		handleUseCaseError(r.Context(), w, err)
		return
	}

	resp := make([]SlackUserInfo, 0, len(users))
	for _, u := range users {
		info := SlackUserInfo{Id: u.ID, Name: u.Name}
		if u.Email != "" {
			info.Email = &u.Email
		}
		if u.ImageURL != "" {
			info.ImageUrl = &u.ImageURL
		}
		resp = append(resp, info)
	}
	writeJSON(r.Context(), w, http.StatusOK, map[string][]SlackUserInfo{"users": resp})
}

func (h *APIHandler) GetSlackUserInfo(w http.ResponseWriter, r *http.Request, workspaceId WorkspaceId, userId string) {
	if h.slackUC == nil {
		errutil.HandleHTTP(r.Context(), w, goerr.New("slack integration not configured"), http.StatusServiceUnavailable)
		return
	}

	info, err := h.slackUC.GetUserInfo(r.Context(), userId)
	if err != nil {
		handleUseCaseError(r.Context(), w, err)
		return
	}

	resp := SlackUserInfo{Id: userId, Name: info.Name}
	if info.Email != "" {
		resp.Email = &info.Email
	}
	if info.ImageURL != "" {
		resp.ImageUrl = &info.ImageURL
	}
	writeJSON(r.Context(), w, http.StatusOK, resp)
}

func (h *APIHandler) ListComments(w http.ResponseWriter, r *http.Request, workspaceId WorkspaceId, ticketId TicketId) {
	comments, err := h.ticketUC.ListComments(r.Context(), types.WorkspaceID(workspaceId), types.TicketID(ticketId))
	if err != nil {
		handleUseCaseError(r.Context(), w, err)
		return
	}

	resp := make([]Comment, 0, len(comments))
	for _, c := range comments {
		resp = append(resp, Comment{
			Id:          string(c.ID),
			SlackUserId: string(c.SlackUserID),
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

	assigneeIDs := make([]string, 0, len(t.AssigneeIDs))
	for _, id := range t.AssigneeIDs {
		assigneeIDs = append(assigneeIDs, string(id))
	}

	ticket := Ticket{
		Id:          string(t.ID),
		SeqNum:      t.SeqNum,
		Title:       t.Title,
		StatusId:    string(t.StatusID),
		AssigneeIds: assigneeIDs,
		Fields:      fields,
		CreatedAt:   t.CreatedAt,
		UpdatedAt:   t.UpdatedAt,
	}

	if t.Description != "" {
		ticket.Description = &t.Description
	}
	if t.ReporterSlackUserID != "" {
		s := string(t.ReporterSlackUserID)
		ticket.ReporterSlackUserId = &s
	}
	if t.SlackChannelID != "" {
		s := string(t.SlackChannelID)
		ticket.SlackChannelId = &s
	}
	if t.SlackThreadTS != "" {
		s := string(t.SlackThreadTS)
		ticket.SlackThreadTs = &s
	}

	return ticket
}

func toWorkspaceConfigResponse(schema *config.FieldSchema) WorkspaceConfig {
	statuses := make([]StatusDef, 0, len(schema.Statuses))
	for _, s := range schema.Statuses {
		statuses = append(statuses, StatusDef{
			Id:       string(s.ID),
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

	closedIDs := make([]string, len(schema.TicketConfig.ClosedStatusIDs))
	for i, id := range schema.TicketConfig.ClosedStatusIDs {
		closedIDs[i] = string(id)
	}

	return WorkspaceConfig{
		Statuses: statuses,
		TicketConfig: TicketConfig{
			DefaultStatusId: string(schema.TicketConfig.DefaultStatusID),
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
		result[string(f.FieldId)] = model.FieldValue{
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
