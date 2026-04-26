package slack

import (
	"context"
	"fmt"
	"strings"

	"github.com/m-mizutani/shepherd/pkg/domain/model"
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
// divider, and one context block per subtask.
func BuildProgressBlocks(ctx context.Context, headerMessage string, subtasks []SubtaskProgress) []slackgo.Block {
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
	return blocks
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

// BuildAskBlocks renders the question form: a header carrying the planner's
// reporter-facing message, one input section per question, and a final
// submit button. block_id of each input section embeds the question id so
// the submission payload can be matched back via state.values.
func BuildAskBlocks(ctx context.Context, ticketID types.TicketID, ask *model.Ask, headerMessage string) []slackgo.Block {
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

		// Free-text fallback ("other") input
		other := slackgo.NewPlainTextInputBlockElement(
			slackgo.NewTextBlockObject(slackgo.PlainTextType, "", false, false),
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
		string(ticketID),
		slackgo.NewTextBlockObject(slackgo.PlainTextType,
			loc.T(i18n.MsgTriageAskSubmitButton),
			false, false),
	).WithStyle(slackgo.StylePrimary)
	blocks = append(blocks, slackgo.NewActionBlock("triage_actions", btn))

	return blocks
}

// BuildAskReceivedBlocks replaces the question form with a single section
// announcing that the answers were accepted. Used after a successful submit
// CAS to remove the input fields and prevent re-submission.
func BuildAskReceivedBlocks(ctx context.Context) []slackgo.Block {
	return []slackgo.Block{
		slackgo.NewSectionBlock(
			slackgo.NewTextBlockObject(slackgo.MarkdownType,
				i18n.From(ctx).T(i18n.MsgTriageAskReceived),
				false, false),
			nil, nil,
		),
	}
}

// BuildAskInvalidatedBlocks replaces the question form when the submission
// arrives after the form is no longer the latest pending Ask (e.g. the LLM
// has already moved on).
func BuildAskInvalidatedBlocks(ctx context.Context) []slackgo.Block {
	return []slackgo.Block{
		slackgo.NewSectionBlock(
			slackgo.NewTextBlockObject(slackgo.MarkdownType,
				i18n.From(ctx).T(i18n.MsgTriageAskInvalidated),
				false, false),
			nil, nil,
		),
	}
}

// BuildAskValidationErrorBlocks rebuilds the question form together with an
// inline error notice so the reporter can adjust their input and resubmit.
func BuildAskValidationErrorBlocks(ctx context.Context, ticketID types.TicketID, ask *model.Ask, headerMessage string) []slackgo.Block {
	blocks := BuildAskBlocks(ctx, ticketID, ask, headerMessage)
	errorBlock := slackgo.NewContextBlock("triage_error",
		slackgo.NewTextBlockObject(slackgo.MarkdownType,
			"⚠️ "+i18n.From(ctx).T(i18n.MsgTriageAskValidationError),
			false, false),
	)
	// Insert validation banner just under the header (index 1, before divider).
	out := make([]slackgo.Block, 0, len(blocks)+1)
	out = append(out, blocks[0], errorBlock)
	out = append(out, blocks[1:]...)
	return out
}

// BuildCompleteBlocks renders the hand-off summary message. When
// AssigneeDecision is Assigned, the message mentions the user; when
// Unassigned, it explains why no individual was picked.
func BuildCompleteBlocks(ctx context.Context, comp *model.Complete) []slackgo.Block {
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

	switch comp.Assignee.Kind {
	case types.AssigneeAssigned:
		if comp.Assignee.UserID != nil {
			blocks = append(blocks, slackgo.NewSectionBlock(
				slackgo.NewTextBlockObject(slackgo.MarkdownType,
					loc.T(i18n.MsgTriageCompleteAssigneeMention, "user", string(*comp.Assignee.UserID)),
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
	if len(comp.KeyFindings) > 0 {
		blocks = append(blocks,
			sectionLabeled(loc.T(i18n.MsgTriageCompleteSectionFindings), bulletList(comp.KeyFindings)),
		)
	}
	if len(comp.AnswerSummary) > 0 {
		blocks = append(blocks,
			sectionLabeled(loc.T(i18n.MsgTriageCompleteSectionAnswers), formatAnswerSummary(comp.AnswerSummary)),
		)
	}
	if len(comp.SimilarTickets) > 0 {
		blocks = append(blocks,
			sectionLabeled(loc.T(i18n.MsgTriageCompleteSectionSimilar), formatTicketIDs(comp.SimilarTickets)),
		)
	}
	if len(comp.NextSteps) > 0 {
		blocks = append(blocks,
			sectionLabeled(loc.T(i18n.MsgTriageCompleteSectionNextSteps), bulletList(comp.NextSteps)),
		)
	}
	return blocks
}

// BuildAbortedBlocks renders the message posted when triage is aborted
// (cap exceeded, panic, permanent error). It records the abort reason so
// operators can investigate.
func BuildAbortedBlocks(ctx context.Context, reason string) []slackgo.Block {
	loc := i18n.From(ctx)
	return []slackgo.Block{
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
}

func sectionLabeled(label, body string) slackgo.Block {
	text := label + "\n" + body
	return slackgo.NewSectionBlock(
		slackgo.NewTextBlockObject(slackgo.MarkdownType, text, false, false),
		nil, nil,
	)
}

func bulletList(items []string) string {
	var b strings.Builder
	for i, item := range items {
		if i > 0 {
			b.WriteString("\n")
		}
		b.WriteString("• ")
		b.WriteString(escapeMrkdwn(item))
	}
	return b.String()
}

func formatAnswerSummary(m map[string]string) string {
	if len(m) == 0 {
		return ""
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	// Stable order helps snapshot tests and human readability.
	sortStrings(keys)
	var b strings.Builder
	for i, k := range keys {
		if i > 0 {
			b.WriteString("\n")
		}
		fmt.Fprintf(&b, "• *%s* — %s", escapeMrkdwn(k), escapeMrkdwn(m[k]))
	}
	return b.String()
}

func formatTicketIDs(ids []types.TicketID) string {
	parts := make([]string, 0, len(ids))
	for _, id := range ids {
		parts = append(parts, "`"+escapeMrkdwn(string(id))+"`")
	}
	return strings.Join(parts, ", ")
}

// escapeMrkdwn escapes Slack mrkdwn metacharacters so user-supplied content
// (LLM output, ticket text) cannot interfere with formatting.
func escapeMrkdwn(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	return s
}

func sortStrings(s []string) {
	// Tiny insertion sort to avoid pulling sort just for tests; n is small.
	for i := 1; i < len(s); i++ {
		j := i
		for j > 0 && s[j] < s[j-1] {
			s[j], s[j-1] = s[j-1], s[j]
			j--
		}
	}
}
