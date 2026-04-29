package slack

import (
	"context"
	"strings"

	"github.com/m-mizutani/shepherd/pkg/domain/model"
	domainConfig "github.com/m-mizutani/shepherd/pkg/domain/model/config"
	"github.com/m-mizutani/shepherd/pkg/domain/types"
	"github.com/m-mizutani/shepherd/pkg/utils/i18n"
	slackgo "github.com/slack-go/slack"
)

// TriageSubmitActionID is the action_id of the submit button on the triage
// question form. The HTTP interactions handler dispatches block_actions with
// this id to the triage usecase.
const TriageSubmitActionID = "triage_submit_answers"

// TriageChoiceActionID is the action_id of the radio_buttons / checkboxes
// element rendering the predefined choices for a single question.
const TriageChoiceActionID = "triage_choice"

// TriageOtherTextActionID is the action_id of the plain_text_input element
// holding the free-form fallback for a question.
const TriageOtherTextActionID = "triage_other_text"

// TriageOtherSuffix is the suffix appended to a question id when forming the
// block_id for its free-text fallback input. Used on submission to pair the
// free-text response with its parent question.
const TriageOtherSuffix = ":other"

// TriageRetryActionID is the action_id of the retry button posted by the
// failure-recovery message when triage exits abnormally. The button's value
// carries the ticket id.
const TriageRetryActionID = "triage_retry"

// SubtaskState is the lifecycle phase of a single subtask in the progress
// message. The triage usecase mutates these states via UpdateBlocks.
type SubtaskState int

const (
	SubtaskQueued SubtaskState = iota
	SubtaskRunning
	SubtaskDone
	SubtaskFailed
)

// SubtaskProgress is one subtask's display row in the progress message.
type SubtaskProgress struct {
	ID      types.SubtaskID
	Request string // shown next to the state icon
	State   SubtaskState
	Trace   string // current trace text when SubtaskRunning
	Error   string // error summary when SubtaskFailed
}

// BuildProgressBlocks renders the progress message for an Investigate
// iteration: a header carrying the planner's reporter-facing message, a
// divider, and one context block per subtask. The ticket reference is
// rendered as Inactive — the per-subtask state icons (⏳🔄✅❌) already
// carry the "currently running" signal, and reserving the Active 🎫 form
// for messages that demand reader attention keeps long threads scannable.
func BuildProgressBlocks(ctx context.Context, ref TicketRef, headerMessage string, subtasks []SubtaskProgress) []slackgo.Block {
	header := slackgo.NewSectionBlock(
		slackgo.NewTextBlockObject(slackgo.MarkdownType,
			i18n.From(ctx).T(i18n.MsgTriageProgressHeader, "message", headerMessage),
			false, false,
		),
		nil, nil,
	)
	blocks := []slackgo.Block{header, slackgo.NewDividerBlock()}

	for _, st := range subtasks {
		text := renderSubtaskLine(ctx, st)
		blocks = append(blocks,
			slackgo.NewContextBlock("subtask:"+string(st.ID),
				slackgo.NewTextBlockObject(slackgo.MarkdownType, text, false, false),
			),
		)
	}
	return prependTicketRef(ctx, ref, TicketRefStateInactive, blocks)
}

func renderSubtaskLine(ctx context.Context, st SubtaskProgress) string {
	loc := i18n.From(ctx)
	switch st.State {
	case SubtaskRunning:
		trace := st.Trace
		if trace == "" {
			trace = "..."
		}
		return loc.T(i18n.MsgTriageProgressRunning, "request", st.Request, "trace", trace)
	case SubtaskDone:
		return loc.T(i18n.MsgTriageProgressDone, "request", st.Request)
	case SubtaskFailed:
		err := st.Error
		if err == "" {
			err = "unknown error"
		}
		return loc.T(i18n.MsgTriageProgressFailed, "request", st.Request, "error", err)
	default:
		return loc.T(i18n.MsgTriageProgressQueued, "request", st.Request)
	}
}

// BuildAskBlocks renders the question form: the shared ticket badge, a
// header carrying the planner's reporter-facing message, one input section
// per question, and a final submit button. block_id of each input section
// embeds the question id so the submission payload can be matched back via
// state.values.
func BuildAskBlocks(ctx context.Context, ref TicketRef, ask *model.Ask, headerMessage string) []slackgo.Block {
	loc := i18n.From(ctx)
	blocks := []slackgo.Block{
		slackgo.NewSectionBlock(
			slackgo.NewTextBlockObject(slackgo.MarkdownType,
				loc.T(i18n.MsgTriageAskHeader, "message", headerMessage),
				false, false,
			),
			nil, nil,
		),
		slackgo.NewDividerBlock(),
	}

	for _, q := range ask.Questions {
		// Question label section
		labelText := "*" + escapeMrkdwn(q.Label) + "*"
		if q.Help != "" {
			labelText += "\n" + escapeMrkdwn(q.Help)
		}
		blocks = append(blocks, slackgo.NewSectionBlock(
			slackgo.NewTextBlockObject(slackgo.MarkdownType, labelText, false, false),
			nil, nil,
		))

		// Choices input
		opts := make([]*slackgo.OptionBlockObject, 0, len(q.Choices))
		for _, c := range q.Choices {
			opts = append(opts, slackgo.NewOptionBlockObject(
				string(c.ID),
				slackgo.NewTextBlockObject(slackgo.PlainTextType, c.Label, false, false),
				nil,
			))
		}
		var choiceElement slackgo.BlockElement
		if q.Multiple {
			choiceElement = slackgo.NewCheckboxGroupsBlockElement(TriageChoiceActionID, opts...)
		} else {
			choiceElement = slackgo.NewRadioButtonsBlockElement(TriageChoiceActionID, opts...)
		}
		choiceInput := slackgo.NewInputBlock(
			string(q.ID),
			slackgo.NewTextBlockObject(slackgo.PlainTextType, " ", false, false),
			nil,
			choiceElement,
		)
		choiceInput.Optional = true // required-vs-optional is enforced server-side per Answer.IsValid.
		blocks = append(blocks, choiceInput)

		// Free-text fallback ("other") input. Placeholder is intentionally
		// nil — Slack rejects plain_text objects with empty `text` (and an
		// empty placeholder is the canonical "invalid_blocks" trigger here).
		other := slackgo.NewPlainTextInputBlockElement(
			nil,
			TriageOtherTextActionID,
		)
		other.Multiline = true
		otherInput := slackgo.NewInputBlock(
			string(q.ID)+TriageOtherSuffix,
			slackgo.NewTextBlockObject(slackgo.PlainTextType,
				loc.T(i18n.MsgTriageAskOtherTextLabel),
				false, false),
			nil,
			other,
		)
		otherInput.Optional = true
		blocks = append(blocks, otherInput)

		blocks = append(blocks, slackgo.NewDividerBlock())
	}

	// Submit button
	btn := slackgo.NewButtonBlockElement(
		TriageSubmitActionID,
		string(ref.ID),
		slackgo.NewTextBlockObject(slackgo.PlainTextType,
			loc.T(i18n.MsgTriageAskSubmitButton),
			false, false),
	).WithStyle(slackgo.StylePrimary)
	blocks = append(blocks, slackgo.NewActionBlock("triage_actions", btn))

	// Active: a live form awaiting the reporter's input is the ticket's
	// current state until they hit Submit (which chat.update's this message
	// to the Inactive AskAnswered form).
	return prependTicketRef(ctx, ref, TicketRefStateActive, blocks)
}

// BuildAskAnsweredBlocks rebuilds the form-equivalent message after a
// successful submit: the header, each question label paired with the
// reporter's answer, and a "received" footer. The interactive controls
// (radios / checkboxes / free-text input / submit button) are intentionally
// omitted — once answered, the form is read-only — but the questions and
// answers stay visible so the thread retains the full Q&A trail rather than
// collapsing to a single "thanks" line.
func BuildAskAnsweredBlocks(ctx context.Context, ref TicketRef, ask *model.Ask, answers []model.Answer, headerMessage string) []slackgo.Block {
	loc := i18n.From(ctx)
	blocks := []slackgo.Block{
		slackgo.NewSectionBlock(
			slackgo.NewTextBlockObject(slackgo.MarkdownType,
				loc.T(i18n.MsgTriageAskHeader, "message", headerMessage),
				false, false,
			),
			nil, nil,
		),
		slackgo.NewDividerBlock(),
	}

	choiceLabels := make(map[types.QuestionID]map[types.ChoiceID]string, len(ask.Questions))
	for _, q := range ask.Questions {
		m := make(map[types.ChoiceID]string, len(q.Choices))
		for _, c := range q.Choices {
			m[c.ID] = c.Label
		}
		choiceLabels[q.ID] = m
	}
	answerByQ := make(map[types.QuestionID]model.Answer, len(answers))
	for _, a := range answers {
		answerByQ[a.QuestionID] = a
	}

	for _, q := range ask.Questions {
		labelText := "*" + escapeMrkdwn(q.Label) + "*"
		if q.Help != "" {
			labelText += "\n" + escapeMrkdwn(q.Help)
		}
		blocks = append(blocks, slackgo.NewSectionBlock(
			slackgo.NewTextBlockObject(slackgo.MarkdownType, labelText, false, false),
			nil, nil,
		))

		ans := answerByQ[q.ID]
		body := renderAnswerBody(loc, ans, choiceLabels[q.ID])
		blocks = append(blocks, slackgo.NewSectionBlock(
			slackgo.NewTextBlockObject(slackgo.MarkdownType, body, false, false),
			nil, nil,
		))
		blocks = append(blocks, slackgo.NewDividerBlock())
	}

	blocks = append(blocks,
		slackgo.NewContextBlock("triage_received",
			slackgo.NewTextBlockObject(slackgo.MarkdownType,
				"✅ "+loc.T(i18n.MsgTriageAskReceived),
				false, false),
		),
	)
	// Inactive: the form has been answered; the live state is whatever
	// planner output follows.
	return prependTicketRef(ctx, ref, TicketRefStateInactive, blocks)
}

func renderAnswerBody(loc i18n.Translator, ans model.Answer, choiceLabels map[types.ChoiceID]string) string {
	var parts []string
	for _, sid := range ans.SelectedIDs {
		if lbl, ok := choiceLabels[sid]; ok {
			parts = append(parts, "• "+escapeMrkdwn(lbl))
		} else {
			parts = append(parts, "• "+escapeMrkdwn(string(sid)))
		}
	}
	if other := strings.TrimSpace(ans.OtherText); other != "" {
		parts = append(parts, "• _"+escapeMrkdwn(other)+"_")
	}
	if len(parts) == 0 {
		return "_" + loc.T(i18n.MsgTriageAskAnswerNone) + "_"
	}
	return strings.Join(parts, "\n")
}

// BuildAskInvalidatedBlocks replaces the question form when the submission
// arrives after the form is no longer the latest pending Ask (e.g. the LLM
// has already moved on).
func BuildAskInvalidatedBlocks(ctx context.Context, ref TicketRef) []slackgo.Block {
	blocks := []slackgo.Block{
		slackgo.NewSectionBlock(
			slackgo.NewTextBlockObject(slackgo.MarkdownType,
				i18n.From(ctx).T(i18n.MsgTriageAskInvalidated),
				false, false),
			nil, nil,
		),
	}
	// Inactive: the original form is no longer the live state.
	return prependTicketRef(ctx, ref, TicketRefStateInactive, blocks)
}

// BuildAskValidationErrorBlocks rebuilds the question form together with an
// inline error notice so the reporter can adjust their input and resubmit.
// The form remains action-required (the reporter still has to submit), so
// BuildAskBlocks already prepends the Active ticket reference; we only
// splice the validation banner in below the leading title + header section
// pair so the reporter sees it immediately on re-render.
func BuildAskValidationErrorBlocks(ctx context.Context, ref TicketRef, ask *model.Ask, headerMessage string) []slackgo.Block {
	blocks := BuildAskBlocks(ctx, ref, ask, headerMessage)
	errorBlock := slackgo.NewContextBlock("triage_error",
		slackgo.NewTextBlockObject(slackgo.MarkdownType,
			"⚠️ "+i18n.From(ctx).T(i18n.MsgTriageAskValidationError),
			false, false),
	)
	// BuildAskBlocks returns [ticketRef?, askHeader, divider, …]. When ref
	// is empty (no SeqNum) the leading ticketRef block is omitted, so we
	// scope the leading prefix length to the ref's presence. The validation
	// banner is then spliced after the prefix so it lands directly below
	// the ask header section without disturbing the body.
	leading := 1
	if ref.SeqNum != 0 {
		leading = 2
	}
	if len(blocks) < leading {
		leading = len(blocks)
	}
	out := make([]slackgo.Block, 0, len(blocks)+1)
	out = append(out, blocks[:leading]...)
	out = append(out, errorBlock)
	out = append(out, blocks[leading:]...)
	return out
}

// BuildCompleteBlocks renders the hand-off summary message. When
// AssigneeDecision is Assigned, the message mentions the user; when
// Unassigned, it explains why no individual was picked. schema +
// fieldValues, when non-nil/non-empty, render the ticket's persisted
// custom field values inline so the assignee sees them on the hand-off.
func BuildCompleteBlocks(ctx context.Context, ref TicketRef, comp *model.Complete, schema *domainConfig.FieldSchema, fieldValues map[string]model.FieldValue) []slackgo.Block {
	loc := i18n.From(ctx)

	var headerKey i18n.MsgKey
	if comp.Assignee.Kind == types.AssigneeAssigned {
		headerKey = i18n.MsgTriageCompleteHeaderAssigned
	} else {
		headerKey = i18n.MsgTriageCompleteHeaderUnassigned
	}

	blocks := []slackgo.Block{
		slackgo.NewHeaderBlock(slackgo.NewTextBlockObject(
			slackgo.PlainTextType, loc.T(headerKey), false, false,
		)),
	}

	if strings.TrimSpace(comp.Title) != "" {
		blocks = append(blocks, slackgo.NewSectionBlock(
			slackgo.NewTextBlockObject(slackgo.MarkdownType,
				"*"+escapeMrkdwn(comp.Title)+"*",
				false, false),
			nil, nil,
		))
	}

	switch comp.Assignee.Kind {
	case types.AssigneeAssigned:
		if mentions := joinAssigneeMentions(comp.Assignee.UserIDs); mentions != "" {
			blocks = append(blocks, slackgo.NewSectionBlock(
				slackgo.NewTextBlockObject(slackgo.MarkdownType,
					loc.T(i18n.MsgTriageCompleteAssigneeMention, "users", mentions),
					false, false),
				nil, nil,
			))
		}
		if comp.Assignee.Reasoning != "" {
			blocks = append(blocks, slackgo.NewContextBlock("triage_reasoning",
				slackgo.NewTextBlockObject(slackgo.MarkdownType,
					"_"+escapeMrkdwn(comp.Assignee.Reasoning)+"_",
					false, false),
			))
		}
	case types.AssigneeUnassigned:
		blocks = append(blocks, slackgo.NewSectionBlock(
			slackgo.NewTextBlockObject(slackgo.MarkdownType,
				loc.T(i18n.MsgTriageCompleteUnassignedReason, "reason", comp.Assignee.Reasoning),
				false, false),
			nil, nil,
		))
	}

	blocks = append(blocks, slackgo.NewDividerBlock())

	if comp.Summary != "" {
		blocks = append(blocks,
			sectionLabeled(loc.T(i18n.MsgTriageCompleteSectionSummary), comp.Summary),
		)
	}
	blocks = append(blocks, fieldValuesBlocks(ctx, schema, fieldValues)...)
	// Active: terminal hand-off summary on the auto-finalise path is the
	// ticket's live state.
	return prependTicketRef(ctx, ref, TicketRefStateActive, blocks)
}

// BuildFailedBlocks renders the recovery message posted when the triage
// planner exits abnormally (run() returned an error or panicked). The
// message includes the error summary and a retry button keyed to the ticket
// id; the HTTP interactions handler dispatches that button to
// UseCase.HandleRetry.
func BuildFailedBlocks(ctx context.Context, ref TicketRef, errMessage string) []slackgo.Block {
	loc := i18n.From(ctx)
	blocks := []slackgo.Block{
		slackgo.NewHeaderBlock(slackgo.NewTextBlockObject(
			slackgo.PlainTextType, loc.T(i18n.MsgTriageFailedHeader), false, false,
		)),
		slackgo.NewSectionBlock(
			slackgo.NewTextBlockObject(slackgo.MarkdownType,
				loc.T(i18n.MsgTriageFailedError, "error", escapeMrkdwn(errMessage)),
				false, false),
			nil, nil,
		),
	}
	btn := slackgo.NewButtonBlockElement(
		TriageRetryActionID,
		string(ref.ID),
		slackgo.NewTextBlockObject(slackgo.PlainTextType,
			loc.T(i18n.MsgTriageFailedRetryButton),
			false, false),
	).WithStyle(slackgo.StylePrimary)
	blocks = append(blocks, slackgo.NewActionBlock("triage_retry_actions", btn))
	// Active: a failure with a Retry button is action-required, the ticket's
	// current live state until someone clicks Retry.
	return prependTicketRef(ctx, ref, TicketRefStateActive, blocks)
}

// BuildRetryQueuedBlocks replaces the failure message after the retry button
// is clicked, so subsequent clicks do not re-queue the planner.
func BuildRetryQueuedBlocks(ctx context.Context, ref TicketRef) []slackgo.Block {
	blocks := []slackgo.Block{
		slackgo.NewSectionBlock(
			slackgo.NewTextBlockObject(slackgo.MarkdownType,
				i18n.From(ctx).T(i18n.MsgTriageRetryQueued),
				false, false),
			nil, nil,
		),
	}
	// Inactive: transitional notice; the resumed planner output is the next
	// live state.
	return prependTicketRef(ctx, ref, TicketRefStateInactive, blocks)
}

// BuildAbortedBlocks renders the message posted when triage is aborted
// (cap exceeded, panic, permanent error). It records the abort reason so
// operators can investigate.
func BuildAbortedBlocks(ctx context.Context, ref TicketRef, reason string) []slackgo.Block {
	loc := i18n.From(ctx)
	blocks := []slackgo.Block{
		slackgo.NewHeaderBlock(slackgo.NewTextBlockObject(
			slackgo.PlainTextType, loc.T(i18n.MsgTriageAbortedHeader), false, false,
		)),
		slackgo.NewSectionBlock(
			slackgo.NewTextBlockObject(slackgo.MarkdownType,
				loc.T(i18n.MsgTriageAbortedReason, "reason", reason),
				false, false),
			nil, nil,
		),
	}
	// Active: terminal aborted state — final live message for this ticket.
	return prependTicketRef(ctx, ref, TicketRefStateActive, blocks)
}

func sectionLabeled(label, body string) slackgo.Block {
	text := label + "\n" + body
	return slackgo.NewSectionBlock(
		slackgo.NewTextBlockObject(slackgo.MarkdownType, text, false, false),
		nil, nil,
	)
}

// joinAssigneeMentions formats assignee Slack ids as space-separated mention
// tokens (e.g. "<@U123> <@U456>"). Empty ids are skipped. Returns "" when no
// valid id is present.
func joinAssigneeMentions(ids []types.SlackUserID) string {
	mentions := make([]string, 0, len(ids))
	for _, id := range ids {
		if id == "" {
			continue
		}
		mentions = append(mentions, "<@"+string(id)+">")
	}
	return strings.Join(mentions, " ")
}

// escapeMrkdwn escapes Slack mrkdwn metacharacters so user-supplied content
// (LLM output, ticket text) cannot interfere with formatting.
func escapeMrkdwn(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	return s
}
