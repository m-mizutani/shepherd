package i18n

type MsgKey string

const (
	MsgTicketCreated     MsgKey = "ticket_created"
	MsgStatusChange      MsgKey = "status_change"
	MsgStatusChangeLabel MsgKey = "status_change_label"

	// Ticket change notification (context block) — emitted whenever a
	// TicketUseCase.Update mutation flips the ticket's status or assignee
	// regardless of which entry point (HTTP API, Slack quick-actions menu,
	// future surfaces) drove the change. The two pieces are interpolated
	// into a single context block so simultaneous status + assignee
	// transitions render as one notification, not two.
	MsgAssigneeChange MsgKey = "assignee_change"
	MsgUnassigned     MsgKey = "unassigned"

	// Slack empty-mention quick-actions menu — posted into the ticket
	// thread when a user @-mentions the bot with no body. Lets the user
	// switch the ticket's assignee or status without leaving Slack.
	MsgQuickActionsHeader            MsgKey = "quick_actions_header"
	MsgQuickActionsAssigneeLabel     MsgKey = "quick_actions_assignee_label"
	MsgQuickActionsStatusLabel       MsgKey = "quick_actions_status_label"
	MsgQuickActionsStatusPlaceholder MsgKey = "quick_actions_status_placeholder"
	MsgQuickActionsAssigneePlaceholder MsgKey = "quick_actions_assignee_placeholder"

	// Three rendering forms for the ticket reference line, applied at the
	// top of every ticket-scoped Slack message. The form is chosen by the
	// message's lifecycle state so a thread reader can tell at a glance
	// which message represents the ticket's current live state vs.
	// historical chatter:
	//   - Active   : the latest live message (e.g. review awaiting click,
	//                ask form awaiting answer, terminal complete). Rendered
	//                bold with the 🎫 marker so it stands out.
	//   - Inactive : historical / transitional messages (progress, hand-off,
	//                submitted, retry-queued, ask-answered, …). Rendered
	//                plain so old messages do not compete for attention.
	//   - Dismissed: the review proposal a user sent back via Re-investigate;
	//                rendered struck-through so the rejected proposal is
	//                visibly invalidated.
	MsgTicketRefActive    MsgKey = "ticket_ref_active"
	MsgTicketRefInactive  MsgKey = "ticket_ref_inactive"
	MsgTicketRefDismissed MsgKey = "ticket_ref_dismissed"

	// Triage progress message (one Slack message per Investigate iteration,
	// holding a context block per subtask).
	MsgTriageProgressHeader  MsgKey = "triage_progress_header"
	MsgTriageProgressQueued  MsgKey = "triage_progress_queued"
	MsgTriageProgressRunning MsgKey = "triage_progress_running"
	MsgTriageProgressDone    MsgKey = "triage_progress_done"
	MsgTriageProgressFailed  MsgKey = "triage_progress_failed"

	// Triage Ask form (the question message and its lifecycle updates).
	MsgTriageAskHeader          MsgKey = "triage_ask_header"
	MsgTriageAskSubmitButton    MsgKey = "triage_ask_submit_button"
	MsgTriageAskOtherTextLabel  MsgKey = "triage_ask_other_text_label"
	MsgTriageAskReceived        MsgKey = "triage_ask_received"
	MsgTriageAskInvalidated     MsgKey = "triage_ask_invalidated"
	MsgTriageAskValidationError MsgKey = "triage_ask_validation_error"
	MsgTriageAskHistoryMissing  MsgKey = "triage_ask_history_missing"
	MsgTriageAskAnswerNone      MsgKey = "triage_ask_answer_none"

	// Triage completion / abort report.
	MsgTriageCompleteHeaderAssigned     MsgKey = "triage_complete_header_assigned"
	MsgTriageCompleteHeaderUnassigned   MsgKey = "triage_complete_header_unassigned"
	MsgTriageCompleteAssigneeMention    MsgKey = "triage_complete_assignee_mention"
	MsgTriageCompleteSectionSummary     MsgKey = "triage_complete_section_summary"
	MsgTriageCompleteSectionFindings    MsgKey = "triage_complete_section_findings"
	MsgTriageCompleteSectionAnswers     MsgKey = "triage_complete_section_answers"
	MsgTriageCompleteSectionSimilar     MsgKey = "triage_complete_section_similar_tickets"
	MsgTriageCompleteSectionNextSteps   MsgKey = "triage_complete_section_next_steps"
	MsgTriageCompleteUnassignedReason   MsgKey = "triage_complete_unassigned_reason"
	MsgTriageAbortedHeader              MsgKey = "triage_aborted_header"
	MsgTriageAbortedReason              MsgKey = "triage_aborted_reason"

	// Triage failure (deferred recovery report) — posted to the ticket thread
	// when run() returns an error or panics. Carries the error summary plus a
	// retry button that re-dispatches the planner.
	MsgTriageFailedHeader      MsgKey = "triage_failed_header"
	MsgTriageFailedError       MsgKey = "triage_failed_error"
	MsgTriageFailedRetryButton MsgKey = "triage_failed_retry_button"
	MsgTriageRetryQueued       MsgKey = "triage_retry_queued"
	// Posted to the ticket thread when the planner's structured response
	// fails validation and is being re-asked. Intentionally vague — the
	// detailed error stays in the operator-facing logs only.
	MsgTriagePlanRetrying MsgKey = "triage_plan_retrying"

	// Triage reporter-review flow (default; opt out per workspace via
	// [triage] auto = true to fall back to immediate finalisation). The
	// review message carries Edit / Submit / Re-investigate buttons; outcomes
	// are posted as additional thread messages instead of rewriting the
	// review message itself.
	MsgTriageReviewMentionRequester             MsgKey = "triage_review_mention_requester"
	MsgTriageReviewBtnEdit                      MsgKey = "triage_review_btn_edit"
	MsgTriageReviewBtnSubmit                    MsgKey = "triage_review_btn_submit"
	MsgTriageReviewBtnReinvestigate             MsgKey = "triage_review_btn_reinvestigate"
	MsgTriageReviewSubmittedHeader              MsgKey = "triage_review_submitted_header"
	MsgTriageReviewReinvestigatingHeader        MsgKey = "triage_review_reinvestigating_header"
	MsgTriageReviewActionedSubmittedFooter      MsgKey = "triage_review_actioned_submitted_footer"
	MsgTriageReviewActionedReinvestigateFooter  MsgKey = "triage_review_actioned_reinvestigate_footer"
	MsgTriageReviewHandoffFallback              MsgKey = "triage_review_handoff_fallback"
	MsgTriageReviewReinvestigatingInstruction   MsgKey = "triage_review_reinvestigating_instruction"
	MsgTriageReviewMentionAssignee              MsgKey = "triage_review_mention_assignee"
	MsgTriageReviewAlreadyFinalized             MsgKey = "triage_review_already_finalized"
	MsgTriageReviewMissingProposal              MsgKey = "triage_review_missing_proposal"
	MsgTriageReviewEditModalTitle               MsgKey = "triage_review_edit_modal_title"
	MsgTriageReviewEditTitleLabel               MsgKey = "triage_review_edit_title_label"
	MsgTriageReviewEditDescriptionLabel         MsgKey = "triage_review_edit_description_label"
	MsgTriageReviewEditAssigneeLabel            MsgKey = "triage_review_edit_assignee_label"
	MsgTriageReviewEditFieldsHeader             MsgKey = "triage_review_edit_fields_header"
	MsgTriageReviewEditModalSubmit              MsgKey = "triage_review_edit_modal_submit"
	MsgTriageReviewEditModalClose               MsgKey = "triage_review_edit_modal_close"
	MsgTriageReviewReinvestigateModalTitle      MsgKey = "triage_review_reinvestigate_modal_title"
	MsgTriageReviewReinvestigateInstructionLabel MsgKey = "triage_review_reinvestigate_instruction_label"
	MsgTriageReviewReinvestigateModalSubmit     MsgKey = "triage_review_reinvestigate_modal_submit"
	MsgTriageReviewReinvestigateModalClose      MsgKey = "triage_review_reinvestigate_modal_close"
	MsgTriageReviewFieldRequiredError           MsgKey = "triage_review_field_required_error"
	MsgTriageReviewFieldSelectPlaceholder       MsgKey = "triage_review_field_select_placeholder"

	// Conclusion message posted to the ticket thread when the ticket
	// transitions to a closed status. Rendered as a single Slack context
	// block with minimal decoration; the leading emoji is hard-coded by the
	// service layer so the LLM output never carries extra ornaments.
	MsgConclusionBody MsgKey = "conclusion_body"
)
