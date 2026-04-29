package slack

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/m-mizutani/goerr/v2"
	"github.com/m-mizutani/shepherd/pkg/domain/model"
	domainConfig "github.com/m-mizutani/shepherd/pkg/domain/model/config"
	"github.com/m-mizutani/shepherd/pkg/domain/types"
	"github.com/m-mizutani/shepherd/pkg/utils/i18n"
	slackgo "github.com/slack-go/slack"
)

// Review-flow Slack identifiers. The action_id values are the contract between
// the Block Kit message and the HTTP interactions handler; the callback_id
// values identify which modal the view_submission payload originates from.
const (
	TriageReviewEditActionID          = "triage_review_edit"
	TriageReviewSubmitActionID        = "triage_review_submit"
	TriageReviewReinvestigateActionID = "triage_review_reinvestigate"

	TriageReviewEditModalCallbackID          = "triage_review_edit_modal"
	TriageReviewReinvestigateModalCallbackID = "triage_review_reinvestigate_modal"

	// Modal field action_ids. Per-question custom-field block_ids are set to
	// the FieldDefinition.ID at render time; their action_ids share this
	// constant so the parser can look them up uniformly.
	TriageReviewTitleBlockID      = "triage_review_title"
	TriageReviewTitleActionID     = "triage_review_title_input"
	TriageReviewSummaryBlockID    = "triage_review_summary"
	TriageReviewSummaryActionID   = "triage_review_summary_input"
	TriageReviewAssigneeBlockID   = "triage_review_assignee"
	TriageReviewAssigneeActionID  = "triage_review_assignee_input"
	TriageReviewFieldValueAction  = "field_value"
	TriageReviewInstructionBlock  = "triage_review_instruction"
	TriageReviewInstructionAction = "triage_review_instruction_input"
)

// TriageReviewModalMetadata is the JSON payload stored in a modal's
// private_metadata so the view_submission handler can map back to the ticket
// without re-encoding state in action values.
type TriageReviewModalMetadata struct {
	TicketID  types.TicketID `json:"ticket_id"`
	ChannelID string         `json:"channel_id"`
	MessageTS string         `json:"message_ts"`
}

// EncodeTriageReviewModalMetadata serialises the metadata as JSON for storage
// in slackgo.ModalViewRequest.PrivateMetadata.
func EncodeTriageReviewModalMetadata(m TriageReviewModalMetadata) (string, error) {
	b, err := json.Marshal(m)
	if err != nil {
		return "", goerr.Wrap(err, "marshal modal metadata")
	}
	return string(b), nil
}

// DecodeTriageReviewModalMetadata reverses EncodeTriageReviewModalMetadata.
func DecodeTriageReviewModalMetadata(raw string) (TriageReviewModalMetadata, error) {
	var m TriageReviewModalMetadata
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		return m, goerr.Wrap(err, "unmarshal modal metadata")
	}
	return m, nil
}

// BuildReviewBlocks renders the planner's PlanComplete proposal with three
// action buttons. The body of the message reuses the same sections as
// BuildCompleteBlocks so reporters see identical content; the difference is
// that finalisation is gated by the buttons. requesterID, when non-empty,
// is mentioned with a fixed call-to-action so the reporter is paged back.
func BuildReviewBlocks(ctx context.Context, ticketID types.TicketID, comp *model.Complete, requesterID types.SlackUserID) []slackgo.Block {
	loc := i18n.From(ctx)

	blocks := []slackgo.Block{
		slackgo.NewHeaderBlock(slackgo.NewTextBlockObject(
			slackgo.PlainTextType, loc.T(i18n.MsgTriageReviewHeader), false, false,
		)),
	}
	if requesterID != "" {
		blocks = append(blocks, slackgo.NewSectionBlock(
			slackgo.NewTextBlockObject(slackgo.MarkdownType,
				loc.T(i18n.MsgTriageReviewMentionRequester, "user", string(requesterID)),
				false, false),
			nil, nil,
		))
	}
	blocks = append(blocks, completeBodyBlocks(ctx, comp)...)
	blocks = append(blocks, slackgo.NewDividerBlock())

	editBtn := slackgo.NewButtonBlockElement(
		TriageReviewEditActionID, string(ticketID),
		slackgo.NewTextBlockObject(slackgo.PlainTextType, loc.T(i18n.MsgTriageReviewBtnEdit), false, false),
	)
	submitBtn := slackgo.NewButtonBlockElement(
		TriageReviewSubmitActionID, string(ticketID),
		slackgo.NewTextBlockObject(slackgo.PlainTextType, loc.T(i18n.MsgTriageReviewBtnSubmit), false, false),
	).WithStyle(slackgo.StylePrimary)
	reinvestigateBtn := slackgo.NewButtonBlockElement(
		TriageReviewReinvestigateActionID, string(ticketID),
		slackgo.NewTextBlockObject(slackgo.PlainTextType, loc.T(i18n.MsgTriageReviewBtnReinvestigate), false, false),
	)

	blocks = append(blocks, slackgo.NewActionBlock("triage_review_actions",
		editBtn, submitBtn, reinvestigateBtn,
	))
	return blocks
}

// ReviewActionedKind identifies why a review message was deactivated.
type ReviewActionedKind int

const (
	// ReviewActionedSubmitted: someone clicked Submit (or Edit + Submit).
	ReviewActionedSubmitted ReviewActionedKind = iota
	// ReviewActionedReinvestigate: someone clicked Re-investigate.
	ReviewActionedReinvestigate
)

// BuildReviewActionedBlocks rebuilds the review message in a finalised /
// neutralised state: same body (so the thread reader still sees what was
// proposed), no buttons, plus a footer line indicating who actioned it. Used
// via chat.update on the original review message so the buttons disappear and
// can no longer be clicked again.
func BuildReviewActionedBlocks(ctx context.Context, comp *model.Complete, kind ReviewActionedKind, actorID types.SlackUserID) []slackgo.Block {
	loc := i18n.From(ctx)
	blocks := []slackgo.Block{
		slackgo.NewHeaderBlock(slackgo.NewTextBlockObject(
			slackgo.PlainTextType, loc.T(i18n.MsgTriageReviewHeader), false, false,
		)),
	}
	blocks = append(blocks, completeBodyBlocks(ctx, comp)...)
	blocks = append(blocks, slackgo.NewDividerBlock())

	var footerKey i18n.MsgKey
	switch kind {
	case ReviewActionedReinvestigate:
		footerKey = i18n.MsgTriageReviewActionedReinvestigateFooter
	default:
		footerKey = i18n.MsgTriageReviewActionedSubmittedFooter
	}
	blocks = append(blocks, slackgo.NewContextBlock("triage_review_actioned",
		slackgo.NewTextBlockObject(slackgo.MarkdownType,
			loc.T(footerKey, "user", string(actorID)),
			false, false),
	))
	return blocks
}

// BuildHandoffMessageBlocks renders the LLM-generated hand-off message that
// is posted as a standalone follow-up after a successful Submit. message is
// expected to already include the assignee mention (the LLM is instructed to
// produce that). When message is empty (LLM failure), the caller should
// substitute the localised fallback so a mention still reaches the assignee.
func BuildHandoffMessageBlocks(_ context.Context, message string) []slackgo.Block {
	return []slackgo.Block{
		slackgo.NewSectionBlock(
			slackgo.NewTextBlockObject(slackgo.MarkdownType, message, false, false),
			nil, nil,
		),
	}
}

// BuildReviewReinvestigatingBlocks renders the follow-up message posted to
// the thread when a Re-investigate is accepted. The instruction text from the
// modal is echoed so the thread records why the planner restarted.
func BuildReviewReinvestigatingBlocks(ctx context.Context, instruction string) []slackgo.Block {
	loc := i18n.From(ctx)
	blocks := []slackgo.Block{
		slackgo.NewHeaderBlock(slackgo.NewTextBlockObject(
			slackgo.PlainTextType, loc.T(i18n.MsgTriageReviewReinvestigatingHeader), false, false,
		)),
	}
	if strings.TrimSpace(instruction) != "" {
		blocks = append(blocks, slackgo.NewSectionBlock(
			slackgo.NewTextBlockObject(slackgo.MarkdownType,
				loc.T(i18n.MsgTriageReviewReinvestigatingInstruction,
					"instruction", escapeMrkdwn(instruction)),
				false, false),
			nil, nil,
		))
	}
	return blocks
}

// BuildReviewEditModal builds the modal that lets a user adjust the planner's
// proposal before submitting. Submitting the modal directly finalises the
// ticket — there is no separate "save edits" state.
func BuildReviewEditModal(ctx context.Context, meta TriageReviewModalMetadata, comp *model.Complete, schema *domainConfig.FieldSchema, fieldValues map[string]model.FieldValue) (slackgo.ModalViewRequest, error) {
	loc := i18n.From(ctx)

	blocks := []slackgo.Block{
		buildTitleInputBlock(ctx, comp.Title),
		buildSummaryInputBlock(ctx, comp.Summary),
		buildAssigneeInputBlock(ctx, comp.Assignee),
	}

	if schema != nil && len(schema.Fields) > 0 {
		blocks = append(blocks,
			slackgo.NewDividerBlock(),
			slackgo.NewSectionBlock(
				slackgo.NewTextBlockObject(slackgo.MarkdownType,
					"*"+loc.T(i18n.MsgTriageReviewEditFieldsHeader)+"*",
					false, false),
				nil, nil,
			),
		)
		for _, f := range schema.Fields {
			block, err := buildFieldInputBlock(ctx, f, comp.SuggestedFields, fieldValues)
			if err != nil {
				return slackgo.ModalViewRequest{}, goerr.Wrap(err, "build field input", goerr.V("field_id", f.ID))
			}
			blocks = append(blocks, block)
		}
	}

	pm, err := EncodeTriageReviewModalMetadata(meta)
	if err != nil {
		return slackgo.ModalViewRequest{}, err
	}

	return slackgo.ModalViewRequest{
		Type:            slackgo.VTModal,
		CallbackID:      TriageReviewEditModalCallbackID,
		Title:           slackgo.NewTextBlockObject(slackgo.PlainTextType, loc.T(i18n.MsgTriageReviewEditModalTitle), false, false),
		Submit:          slackgo.NewTextBlockObject(slackgo.PlainTextType, loc.T(i18n.MsgTriageReviewEditModalSubmit), false, false),
		Close:           slackgo.NewTextBlockObject(slackgo.PlainTextType, loc.T(i18n.MsgTriageReviewEditModalClose), false, false),
		Blocks:          slackgo.Blocks{BlockSet: blocks},
		PrivateMetadata: pm,
	}, nil
}

// BuildReviewReinvestigateModal builds the modal that captures the user's
// follow-up instruction for the planner.
func BuildReviewReinvestigateModal(ctx context.Context, meta TriageReviewModalMetadata) (slackgo.ModalViewRequest, error) {
	loc := i18n.From(ctx)

	input := slackgo.NewPlainTextInputBlockElement(nil, TriageReviewInstructionAction)
	input.Multiline = true
	instructionBlock := slackgo.NewInputBlock(
		TriageReviewInstructionBlock,
		slackgo.NewTextBlockObject(slackgo.PlainTextType, loc.T(i18n.MsgTriageReviewReinvestigateInstructionLabel), false, false),
		nil,
		input,
	)

	pm, err := EncodeTriageReviewModalMetadata(meta)
	if err != nil {
		return slackgo.ModalViewRequest{}, err
	}

	return slackgo.ModalViewRequest{
		Type:            slackgo.VTModal,
		CallbackID:      TriageReviewReinvestigateModalCallbackID,
		Title:           slackgo.NewTextBlockObject(slackgo.PlainTextType, loc.T(i18n.MsgTriageReviewReinvestigateModalTitle), false, false),
		Submit:          slackgo.NewTextBlockObject(slackgo.PlainTextType, loc.T(i18n.MsgTriageReviewReinvestigateModalSubmit), false, false),
		Close:           slackgo.NewTextBlockObject(slackgo.PlainTextType, loc.T(i18n.MsgTriageReviewReinvestigateModalClose), false, false),
		Blocks:          slackgo.Blocks{BlockSet: []slackgo.Block{instructionBlock}},
		PrivateMetadata: pm,
	}, nil
}

// completeBodyBlocks reuses the per-section rendering of BuildCompleteBlocks
// so the Review and Submitted messages mirror the legacy hand-off summary.
//
// Title (when present) is rendered as a sub-header at the top so the reader
// sees the same headline that ticket.Title carries. Summary is rendered as
// the body section labelled with MsgTriageCompleteSectionSummary; finalize
// also writes Summary into ticket.Description.
func completeBodyBlocks(ctx context.Context, comp *model.Complete) []slackgo.Block {
	loc := i18n.From(ctx)
	var blocks []slackgo.Block

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
		if comp.Assignee.UserID != nil {
			blocks = append(blocks, slackgo.NewSectionBlock(
				slackgo.NewTextBlockObject(slackgo.MarkdownType,
					loc.T(i18n.MsgTriageCompleteAssigneeMention, "user", string(*comp.Assignee.UserID)),
					false, false),
				nil, nil,
			))
		}
		if comp.Assignee.Reasoning != "" {
			blocks = append(blocks, slackgo.NewContextBlock("triage_review_reasoning",
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
		blocks = append(blocks, sectionLabeled(loc.T(i18n.MsgTriageCompleteSectionSummary), comp.Summary))
	}
	return blocks
}

func buildTitleInputBlock(ctx context.Context, initial string) slackgo.Block {
	loc := i18n.From(ctx)
	input := slackgo.NewPlainTextInputBlockElement(nil, TriageReviewTitleActionID)
	if strings.TrimSpace(initial) != "" {
		input.InitialValue = initial
	}
	return slackgo.NewInputBlock(
		TriageReviewTitleBlockID,
		slackgo.NewTextBlockObject(slackgo.PlainTextType, loc.T(i18n.MsgTriageReviewEditTitleLabel), false, false),
		nil,
		input,
	)
}

func buildSummaryInputBlock(ctx context.Context, initial string) slackgo.Block {
	loc := i18n.From(ctx)
	input := slackgo.NewPlainTextInputBlockElement(nil, TriageReviewSummaryActionID)
	input.Multiline = true
	// MaxLength is well above any realistic LLM summary; we set it so the
	// hidden truncate bar Slack shows at the bottom-right reads "0/4000"
	// instead of "0/100", subtly hinting the field is meant for long text.
	// Note: Block Kit has no API to specify initial visible row count, so
	// height still autofits to InitialValue.
	maxLen := 4000
	input.MaxLength = maxLen
	if strings.TrimSpace(initial) != "" {
		input.InitialValue = initial
	}
	return slackgo.NewInputBlock(
		TriageReviewSummaryBlockID,
		slackgo.NewTextBlockObject(slackgo.PlainTextType, loc.T(i18n.MsgTriageReviewEditSummaryLabel), false, false),
		nil,
		input,
	)
}

func buildAssigneeInputBlock(ctx context.Context, decision model.AssigneeDecision) slackgo.Block {
	loc := i18n.From(ctx)
	users := slackgo.NewOptionsSelectBlockElement(slackgo.OptTypeUser, nil, TriageReviewAssigneeActionID)
	if decision.Kind == types.AssigneeAssigned && decision.UserID != nil {
		users = users.WithInitialUser(string(*decision.UserID))
	}
	block := slackgo.NewInputBlock(
		TriageReviewAssigneeBlockID,
		slackgo.NewTextBlockObject(slackgo.PlainTextType, loc.T(i18n.MsgTriageReviewEditAssigneeLabel), false, false),
		nil,
		users,
	)
	block.Optional = true
	return block
}

// buildFieldInputBlock maps a custom FieldDefinition to its Slack input
// element. The block_id is set to the FieldDefinition.ID so the parser can
// loop over schema.Fields and look up state.values[field.ID][field_value].
func buildFieldInputBlock(_ context.Context, f domainConfig.FieldDefinition, suggested map[string]string, current map[string]model.FieldValue) (slackgo.Block, error) {
	label := slackgo.NewTextBlockObject(slackgo.PlainTextType, f.Name, false, false)
	hint := (*slackgo.TextBlockObject)(nil)
	if f.Description != "" {
		hint = slackgo.NewTextBlockObject(slackgo.PlainTextType, f.Description, false, false)
	}

	initialString := initialFieldString(f, suggested, current)

	var element slackgo.BlockElement
	switch f.Type {
	case types.FieldTypeText:
		input := slackgo.NewPlainTextInputBlockElement(nil, TriageReviewFieldValueAction)
		input.Multiline = true
		if initialString != "" {
			input.InitialValue = initialString
		}
		element = input
	case types.FieldTypeNumber:
		input := slackgo.NewNumberInputBlockElement(nil, TriageReviewFieldValueAction, true)
		if initialString != "" {
			input.InitialValue = initialString
		}
		element = input
	case types.FieldTypeURL:
		input := slackgo.NewURLTextInputBlockElement(nil, TriageReviewFieldValueAction)
		if initialString != "" {
			input.InitialValue = initialString
		}
		element = input
	case types.FieldTypeDate:
		input := slackgo.NewDatePickerBlockElement(TriageReviewFieldValueAction)
		if initialString != "" {
			input.InitialDate = initialString
		}
		element = input
	case types.FieldTypeSelect:
		opts := buildSelectOptions(f.Options)
		sel := slackgo.NewOptionsSelectBlockElement(slackgo.OptTypeStatic, nil, TriageReviewFieldValueAction, opts...)
		if initialString != "" {
			if opt := findOption(opts, initialString); opt != nil {
				sel = sel.WithInitialOption(opt)
			}
		}
		element = sel
	case types.FieldTypeMultiSelect:
		opts := buildSelectOptions(f.Options)
		sel := slackgo.NewOptionsMultiSelectBlockElement(slackgo.MultiOptTypeStatic, nil, TriageReviewFieldValueAction, opts...)
		initials := initialFieldStrings(f, suggested, current)
		if len(initials) > 0 {
			selected := make([]*slackgo.OptionBlockObject, 0, len(initials))
			for _, id := range initials {
				if opt := findOption(opts, id); opt != nil {
					selected = append(selected, opt)
				}
			}
			if len(selected) > 0 {
				sel = sel.WithInitialOptions(selected...)
			}
		}
		element = sel
	case types.FieldTypeUser:
		sel := slackgo.NewOptionsSelectBlockElement(slackgo.OptTypeUser, nil, TriageReviewFieldValueAction)
		if initialString != "" {
			sel = sel.WithInitialUser(initialString)
		}
		element = sel
	case types.FieldTypeMultiUser:
		sel := slackgo.NewOptionsMultiSelectBlockElement(slackgo.MultiOptTypeUser, nil, TriageReviewFieldValueAction)
		initials := initialFieldStrings(f, suggested, current)
		if len(initials) > 0 {
			sel = sel.WithInitialUsers(initials...)
		}
		element = sel
	default:
		return nil, goerr.New("unsupported field type for review modal", goerr.V("field_type", string(f.Type)))
	}

	block := slackgo.NewInputBlock(f.ID, label, hint, element)
	block.Optional = !f.Required
	return block, nil
}

func buildSelectOptions(opts []domainConfig.FieldOption) []*slackgo.OptionBlockObject {
	out := make([]*slackgo.OptionBlockObject, 0, len(opts))
	for _, o := range opts {
		out = append(out, slackgo.NewOptionBlockObject(
			o.ID,
			slackgo.NewTextBlockObject(slackgo.PlainTextType, o.Name, false, false),
			nil,
		))
	}
	return out
}

func findOption(opts []*slackgo.OptionBlockObject, id string) *slackgo.OptionBlockObject {
	for _, o := range opts {
		if o.Value == id {
			return o
		}
	}
	return nil
}

// initialFieldString picks a single initial value for a scalar field input.
// The current ticket value (if any) wins; the planner's suggestion is the
// fallback so reporters see something even on first render.
func initialFieldString(f domainConfig.FieldDefinition, suggested map[string]string, current map[string]model.FieldValue) string {
	if cur, ok := current[f.ID]; ok {
		if s, ok := scalarToString(cur.Value); ok {
			return s
		}
	}
	if v, ok := suggested[f.ID]; ok {
		return v
	}
	return ""
}

// initialFieldStrings is the multi-select / multi-user counterpart. It splits
// suggested values by comma when no current value exists, matching the
// LLM-side convention of comma-separated id lists.
func initialFieldStrings(f domainConfig.FieldDefinition, suggested map[string]string, current map[string]model.FieldValue) []string {
	if cur, ok := current[f.ID]; ok {
		if list, ok := stringsFromAny(cur.Value); ok {
			return list
		}
	}
	if v, ok := suggested[f.ID]; ok && v != "" {
		parts := strings.Split(v, ",")
		out := make([]string, 0, len(parts))
		for _, p := range parts {
			if s := strings.TrimSpace(p); s != "" {
				out = append(out, s)
			}
		}
		return out
	}
	return nil
}

func scalarToString(v any) (string, bool) {
	switch x := v.(type) {
	case string:
		return x, true
	case fmt.Stringer:
		return x.String(), true
	}
	return "", false
}

func stringsFromAny(v any) ([]string, bool) {
	switch x := v.(type) {
	case []string:
		return x, true
	case []any:
		out := make([]string, 0, len(x))
		for _, e := range x {
			if s, ok := scalarToString(e); ok {
				out = append(out, s)
			}
		}
		return out, true
	}
	return nil, false
}
