package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/m-mizutani/goerr/v2"
	"github.com/m-mizutani/gollem"
	"github.com/m-mizutani/shepherd/pkg/domain/interfaces"
	"github.com/m-mizutani/shepherd/pkg/domain/model"
	"github.com/m-mizutani/shepherd/pkg/domain/model/auth"
	"github.com/m-mizutani/shepherd/pkg/domain/types"
	slackService "github.com/m-mizutani/shepherd/pkg/service/slack"
	"github.com/m-mizutani/shepherd/pkg/usecase/prompt"
	"github.com/m-mizutani/shepherd/pkg/utils/async"
	"github.com/m-mizutani/shepherd/pkg/utils/errutil"
	"github.com/m-mizutani/shepherd/pkg/utils/i18n"
	"github.com/m-mizutani/shepherd/pkg/utils/logging"
)

// ErrConclusionEditNotAllowed is returned when a conclusion edit is requested
// against a ticket that is not currently in a closed status, or when a single
// Update request tries to flip the status and rewrite the conclusion at the
// same time. The HTTP layer translates it to 409 Conflict.
var ErrConclusionEditNotAllowed = goerr.New("conclusion can only be edited while the ticket is closed",
	goerr.Tag(errutil.TagConflict))

// TicketChangeNotifier delivers context-block notifications about ticket
// mutations back to the originating Slack thread.
//
// NotifyTicketChange renders the immediate status / assignee transition
// summary; PostConclusion renders the close-time LLM-generated conclusion.
// They are grouped on a single interface because both represent
// post-update reflections back to the same thread, and the production
// *slackService.Client implements both.
type TicketChangeNotifier interface {
	NotifyTicketChange(ctx context.Context, channelID, threadTS string, change slackService.TicketChange) error
	PostConclusion(ctx context.Context, channelID, threadTS, conclusion string) error
}

type TicketUseCase struct {
	repo     interfaces.Repository
	registry *model.WorkspaceRegistry
	notifier TicketChangeNotifier
	llm      gollem.LLMClient
}

// NewTicketUseCase wires the usecase dependencies. The llm argument is
// optional: when nil, close-time conclusion generation is skipped (status
// updates and notifications still run as usual).
func NewTicketUseCase(repo interfaces.Repository, registry *model.WorkspaceRegistry, notifier TicketChangeNotifier, llm gollem.LLMClient) *TicketUseCase {
	return &TicketUseCase{
		repo:     repo,
		registry: registry,
		notifier: notifier,
		llm:      llm,
	}
}

func (uc *TicketUseCase) Create(ctx context.Context, workspaceID types.WorkspaceID, title, description string, statusID types.StatusID, assigneeIDs []types.SlackUserID, fields map[string]model.FieldValue) (*model.Ticket, error) {
	entry, ok := uc.registry.Get(workspaceID)
	if !ok {
		return nil, goerr.New("workspace not found", goerr.V("workspace_id", workspaceID), goerr.Tag(errutil.TagNotFound))
	}

	if statusID == "" {
		statusID = entry.FieldSchema.TicketConfig.DefaultStatusID
	}

	now := time.Now()
	ticket := &model.Ticket{
		ID:          types.TicketID(uuid.Must(uuid.NewV7()).String()),
		WorkspaceID: workspaceID,
		Title:       title,
		Description: description,
		StatusID:    statusID,
		AssigneeIDs: append([]types.SlackUserID(nil), assigneeIDs...),
		FieldValues: fields,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	created, err := uc.repo.Ticket().Create(ctx, workspaceID, ticket)
	if err != nil {
		return nil, goerr.Wrap(err, "failed to create ticket")
	}

	changedBy := changedByFromContext(ctx)
	history := &model.TicketHistory{
		ID:          uuid.Must(uuid.NewV7()).String(),
		NewStatusID: statusID,
		ChangedBy:   changedBy,
		Action:      "created",
		CreatedAt:   now,
	}
	if _, err := uc.repo.TicketHistory().Create(ctx, workspaceID, created.ID, history); err != nil {
		errutil.Handle(ctx, goerr.Wrap(err, "failed to create ticket history"))
	}

	return created, nil
}

func (uc *TicketUseCase) Get(ctx context.Context, workspaceID types.WorkspaceID, ticketID types.TicketID) (*model.Ticket, error) {
	ticket, err := uc.repo.Ticket().Get(ctx, workspaceID, ticketID)
	if err != nil {
		return nil, goerr.Wrap(err, "failed to get ticket")
	}
	return ticket, nil
}

func (uc *TicketUseCase) List(ctx context.Context, workspaceID types.WorkspaceID, isClosed *bool, statusIDs []types.StatusID) ([]*model.Ticket, error) {
	tickets, err := uc.repo.Ticket().List(ctx, workspaceID, statusIDs)
	if err != nil {
		return nil, goerr.Wrap(err, "failed to list tickets")
	}

	if isClosed != nil {
		entry, ok := uc.registry.Get(workspaceID)
		if ok {
			filtered := tickets[:0]
			for _, t := range tickets {
				if entry.FieldSchema.IsClosedStatus(t.StatusID) == *isClosed {
					filtered = append(filtered, t)
				}
			}
			tickets = filtered
		}
	}

	return tickets, nil
}

func (uc *TicketUseCase) Update(ctx context.Context, workspaceID types.WorkspaceID, ticketID types.TicketID, title, description *string, statusID *types.StatusID, assigneeIDs *[]types.SlackUserID, fields map[string]model.FieldValue, conclusion *string) (*model.Ticket, error) {
	existing, err := uc.repo.Ticket().Get(ctx, workspaceID, ticketID)
	if err != nil {
		return nil, goerr.Wrap(err, "failed to get ticket for update")
	}

	entry, _ := uc.registry.Get(workspaceID)

	if conclusion != nil {
		// Manual conclusion edits are only permitted while the ticket is
		// currently in a closed status. Combining a status flip with a
		// conclusion rewrite in the same request is also rejected to avoid
		// races with the auto-generation path.
		if statusID != nil {
			return nil, ErrConclusionEditNotAllowed
		}
		if entry == nil || !entry.FieldSchema.IsClosedStatus(existing.StatusID) {
			return nil, ErrConclusionEditNotAllowed
		}
	}

	oldStatusID := existing.StatusID
	oldAssigneeIDs := append([]types.SlackUserID(nil), existing.AssigneeIDs...)

	if title != nil {
		existing.Title = *title
	}
	if description != nil {
		existing.Description = *description
	}
	if statusID != nil {
		existing.StatusID = *statusID
	}
	if assigneeIDs != nil {
		existing.AssigneeIDs = append([]types.SlackUserID(nil), (*assigneeIDs)...)
	}
	if fields != nil {
		if existing.FieldValues == nil {
			existing.FieldValues = make(map[string]model.FieldValue)
		}
		for k, v := range fields {
			existing.FieldValues[k] = v
		}
	}
	if conclusion != nil {
		existing.Conclusion = *conclusion
	}
	existing.UpdatedAt = time.Now()

	updated, err := uc.repo.Ticket().Update(ctx, workspaceID, existing)
	if err != nil {
		return nil, goerr.Wrap(err, "failed to update ticket")
	}

	statusChanged := oldStatusID != updated.StatusID
	assigneeChanged := !sameSlackUserIDSet(oldAssigneeIDs, updated.AssigneeIDs)

	if statusChanged {
		changedBy := changedByFromContext(ctx)
		history := &model.TicketHistory{
			ID:          uuid.Must(uuid.NewV7()).String(),
			NewStatusID: updated.StatusID,
			OldStatusID: oldStatusID,
			ChangedBy:   changedBy,
			Action:      "changed",
			CreatedAt:   time.Now(),
		}
		if _, err := uc.repo.TicketHistory().Create(ctx, workspaceID, ticketID, history); err != nil {
			errutil.Handle(ctx, goerr.Wrap(err, "failed to create ticket history"))
		}
	}

	if statusChanged || assigneeChanged {
		uc.notifyTicketChange(ctx, workspaceID, updated, oldStatusID, oldAssigneeIDs, statusChanged, assigneeChanged)
	}

	// Detect non-closed → closed transition and dispatch the conclusion
	// generation. This is the single hook regardless of whether the change
	// originated from a Slack QuickAction or the web PATCH endpoint, so both
	// surfaces inherit the behaviour identically.
	if statusChanged && entry != nil &&
		entry.FieldSchema.IsClosedStatus(updated.StatusID) &&
		!entry.FieldSchema.IsClosedStatus(oldStatusID) {
		wsCopy := workspaceID
		idCopy := updated.ID
		async.Dispatch(ctx, func(ctx context.Context) error {
			return uc.generateConclusion(ctx, wsCopy, idCopy)
		})
	}

	return updated, nil
}

// generateConclusion runs in the async tail after a ticket transitions into
// a closed status. It re-loads the ticket (the request goroutine has already
// acked / returned), bails out when the status was rolled back before the
// LLM ran, asks the LLM for a short prose summary, persists it, and posts a
// single Slack context block back to the originating thread.
//
// Failures along the way are logged via errutil.Handle so the close action
// itself stays "successful" — a missing conclusion is recoverable on the
// next close transition or via manual edit, while pretending the close
// failed would confuse the user.
func (uc *TicketUseCase) generateConclusion(ctx context.Context, workspaceID types.WorkspaceID, ticketID types.TicketID) error {
	if uc.llm == nil {
		return nil
	}
	logger := logging.From(ctx)

	entry, ok := uc.registry.Get(workspaceID)
	if !ok {
		return nil
	}

	ticket, err := uc.repo.Ticket().Get(ctx, workspaceID, ticketID)
	if err != nil {
		errutil.Handle(ctx, goerr.Wrap(err, "load ticket for conclusion generation",
			goerr.V("workspace_id", string(workspaceID)),
			goerr.V("ticket_id", string(ticketID)),
		))
		return nil
	}
	if ticket == nil {
		return nil
	}
	if !entry.FieldSchema.IsClosedStatus(ticket.StatusID) {
		// Lost a race with a re-open; nothing to summarise.
		return nil
	}

	comments, err := uc.repo.Comment().List(ctx, workspaceID, ticketID)
	if err != nil {
		errutil.Handle(ctx, goerr.Wrap(err, "list comments for conclusion generation",
			goerr.V("ticket_id", string(ticketID)),
		))
		return nil
	}

	body, err := buildConclusion(ctx, uc.llm, ticket, comments)
	if err != nil {
		errutil.Handle(ctx, goerr.Wrap(err, "build conclusion",
			goerr.V("ticket_id", string(ticketID)),
		))
		return nil
	}
	if body == "" {
		logger.Warn("conclusion generation produced empty body",
			slog.String("ticket_id", string(ticketID)))
		return nil
	}

	// Persist through the canonical Update path so we re-load the ticket
	// inside Update (the LLM call may have taken seconds while operators
	// edited title / description / fields concurrently — a direct
	// read-modify-write here would clobber those edits with stale data).
	// Routing through Update also makes the close-time conclusion writer
	// itself honour the "Entry-point unification" rule, and re-open races
	// surface as ErrConclusionEditNotAllowed which we treat as a no-op.
	updated, err := uc.Update(ctx, workspaceID, ticketID, nil, nil, nil, nil, nil, &body)
	if err != nil {
		if errors.Is(err, ErrConclusionEditNotAllowed) {
			return nil
		}
		errutil.Handle(ctx, goerr.Wrap(err, "persist generated conclusion",
			goerr.V("ticket_id", string(ticketID)),
		))
		return nil
	}

	if uc.notifier == nil || updated.SlackChannelID == "" || updated.SlackThreadTS == "" {
		return nil
	}
	if err := uc.notifier.PostConclusion(ctx, string(updated.SlackChannelID), string(updated.SlackThreadTS), body); err != nil {
		errutil.Handle(ctx, goerr.Wrap(err, "post conclusion to slack",
			goerr.V("ticket_id", string(ticketID)),
		))
	}
	return nil
}

// conclusionRetryCap bounds how many extra LLM round-trips we are willing
// to spend coaxing a parseable, non-empty conclusion out of the model when
// the first attempt comes back malformed. The cap is small enough that
// total wall-clock stays bounded for a background async task, but large
// enough that occasional JSON glitches self-heal without giving up.
const conclusionRetryCap = 3

// buildConclusion renders the prompt, asks the LLM for a JSON-shaped
// response, and returns the model's prose. The gollem layer enforces the
// schema, but the response can still come back as malformed JSON or with
// an empty `conclusion` field — both are validated here, with up to
// conclusionRetryCap re-asks that hand the verbatim error back to the
// model so it can self-correct on the same gollem session.
func buildConclusion(ctx context.Context, llm gollem.LLMClient, ticket *model.Ticket, comments []*model.Comment) (string, error) {
	in := prompt.ConclusionInput{
		Title:          ticket.Title,
		Description:    ticket.Description,
		InitialMessage: ticket.InitialMessage,
		Comments:       conclusionComments(comments),
		Language:       conclusionLanguage(ctx),
	}
	systemPrompt, err := prompt.RenderConclusion(in)
	if err != nil {
		return "", goerr.Wrap(err, "render conclusion prompt")
	}

	agent := gollem.New(llm,
		gollem.WithSystemPrompt(systemPrompt),
		gollem.WithContentType(gollem.ContentTypeJSON),
		gollem.WithResponseSchema(conclusionResponseSchema()),
	)

	logger := logging.From(ctx)
	kickoff := gollem.Text("Write the conclusion now and return it as the JSON object specified above.")

	for attempt := 0; attempt <= conclusionRetryCap; attempt++ {
		resp, err := agent.Execute(ctx, kickoff)
		if err != nil {
			return "", goerr.Wrap(err, "agent execute",
				goerr.V("ticket_id", string(ticket.ID)),
				goerr.V("attempt", attempt))
		}

		body, validationErr := decodeConclusionResponse(resp)
		if validationErr == nil {
			return body, nil
		}

		if attempt == conclusionRetryCap {
			return "", goerr.Wrap(validationErr, "validate conclusion response",
				goerr.V("ticket_id", string(ticket.ID)),
				goerr.V("attempt", attempt))
		}

		logger.Warn("conclusion response invalid; re-asking",
			slog.String("ticket_id", string(ticket.ID)),
			slog.Int("attempt", attempt),
			slog.Any("error", validationErr),
		)

		// Feed the verbatim error back as the next turn's user message so
		// the model self-corrects on the same gollem session. The previous
		// (invalid) assistant turn already lives in agent history.
		kickoff = gollem.Text(fmt.Sprintf(
			"Your previous response was invalid: %s. Re-emit a JSON object that strictly matches the schema, with a non-empty `conclusion` field, following every style rule from the system prompt.",
			validationErr.Error(),
		))
	}

	// Unreachable: the loop returns from inside on every path.
	return "", goerr.New("conclusion retry loop exited unexpectedly")
}

// decodeConclusionResponse extracts the `conclusion` field from the LLM's
// raw response, applying the validations whose violations should drive a
// retry: empty / nil response, malformed JSON, missing or whitespace-only
// `conclusion`. It returns (trimmed body, nil) on success, or ("", err)
// otherwise.
func decodeConclusionResponse(resp *gollem.ExecuteResponse) (string, error) {
	if resp == nil || len(resp.Texts) == 0 {
		return "", goerr.New("LLM returned no conclusion body")
	}
	raw := strings.Join(resp.Texts, "")

	var out struct {
		Conclusion string `json:"conclusion"`
	}
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return "", goerr.Wrap(err, "decode conclusion json", goerr.V("raw", raw))
	}

	body := strings.TrimSpace(out.Conclusion)
	if body == "" {
		return "", goerr.New("LLM returned empty conclusion field", goerr.V("raw", raw))
	}
	return body, nil
}

func conclusionComments(in []*model.Comment) []prompt.ConclusionComment {
	out := make([]prompt.ConclusionComment, 0, len(in))
	for _, c := range in {
		if c == nil {
			continue
		}
		out = append(out, prompt.ConclusionComment{
			Author: string(c.SlackUserID),
			Body:   c.Body,
		})
	}
	return out
}

func conclusionLanguage(ctx context.Context) string {
	switch i18n.From(ctx).Lang() {
	case i18n.LangJA:
		return "Japanese"
	default:
		return "English"
	}
}

// conclusionResponseSchema is the JSON Schema gollem enforces on the model's
// output. It mirrors the inline shape buildConclusion decodes.
func conclusionResponseSchema() *gollem.Parameter {
	return &gollem.Parameter{
		Title:       "TicketConclusion",
		Description: "The close-time conclusion summarising the ticket.",
		Type:        gollem.TypeObject,
		Properties: map[string]*gollem.Parameter{
			"conclusion": {
				Type:        gollem.TypeString,
				Description: "Short prose summary; no markdown decoration, no emoji.",
				Required:    true,
			},
		},
	}
}

func (uc *TicketUseCase) notifyTicketChange(ctx context.Context, workspaceID types.WorkspaceID, ticket *model.Ticket, oldStatusID types.StatusID, oldAssigneeIDs []types.SlackUserID, statusChanged, assigneeChanged bool) {
	if uc.notifier == nil || ticket.SlackChannelID == "" || ticket.SlackThreadTS == "" {
		return
	}

	entry, ok := uc.registry.Get(workspaceID)
	if !ok {
		return
	}

	change := slackService.TicketChange{
		StatusChanged:   statusChanged,
		AssigneeChanged: assigneeChanged,
	}
	if statusChanged {
		change.OldStatusName = statusName(entry, oldStatusID)
		change.NewStatusName = statusName(entry, ticket.StatusID)
	}
	if assigneeChanged {
		change.OldAssigneeIDs = toUserIDStrings(oldAssigneeIDs)
		change.NewAssigneeIDs = toUserIDStrings(ticket.AssigneeIDs)
	}

	logger := logging.From(ctx)
	if err := uc.notifier.NotifyTicketChange(ctx, string(ticket.SlackChannelID), string(ticket.SlackThreadTS), change); err != nil {
		logger.Warn("failed to notify ticket change to slack",
			slog.String("ticket_id", string(ticket.ID)),
			slog.Any("error", err),
		)
	}
}

func toUserIDStrings(ids []types.SlackUserID) []string {
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		out = append(out, string(id))
	}
	return out
}

// sameSlackUserIDSet treats assignee lists as unordered sets — order in
// AssigneeIDs is not meaningful, so a reorder alone must not trigger a
// "changed" notification.
func sameSlackUserIDSet(a, b []types.SlackUserID) bool {
	if len(a) != len(b) {
		return false
	}
	seen := make(map[types.SlackUserID]int, len(a))
	for _, id := range a {
		seen[id]++
	}
	for _, id := range b {
		seen[id]--
		if seen[id] < 0 {
			return false
		}
	}
	return true
}

func statusName(entry *model.WorkspaceEntry, statusID types.StatusID) string {
	for _, s := range entry.FieldSchema.Statuses {
		if s.ID == statusID {
			return s.Name
		}
	}
	return string(statusID)
}

func (uc *TicketUseCase) Delete(ctx context.Context, workspaceID types.WorkspaceID, ticketID types.TicketID) error {
	if err := uc.repo.Ticket().Delete(ctx, workspaceID, ticketID); err != nil {
		return goerr.Wrap(err, "failed to delete ticket")
	}
	return nil
}

func (uc *TicketUseCase) ListComments(ctx context.Context, workspaceID types.WorkspaceID, ticketID types.TicketID) ([]*model.Comment, error) {
	comments, err := uc.repo.Comment().List(ctx, workspaceID, ticketID)
	if err != nil {
		return nil, goerr.Wrap(err, "failed to list comments")
	}
	return comments, nil
}

func (uc *TicketUseCase) ListHistory(ctx context.Context, workspaceID types.WorkspaceID, ticketID types.TicketID) ([]*model.TicketHistory, error) {
	histories, err := uc.repo.TicketHistory().List(ctx, workspaceID, ticketID)
	if err != nil {
		return nil, goerr.Wrap(err, "failed to list ticket history")
	}
	return histories, nil
}

func changedByFromContext(ctx context.Context) types.SlackUserID {
	token, err := auth.TokenFromContext(ctx)
	if err != nil || token == nil {
		return "system"
	}
	return types.SlackUserID(token.Sub)
}
