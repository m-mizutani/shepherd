package i18n

type MsgKey string

const (
	MsgTicketCreated     MsgKey = "ticket_created"
	MsgStatusChange      MsgKey = "status_change"
	MsgStatusChangeLabel MsgKey = "status_change_label"

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

	// Triage reporter-review flow (default; opt out per workspace via
	// [triage] auto = true to fall back to immediate finalisation). The
	// review message carries Edit / Submit / Re-investigate buttons; outcomes
	// are posted as additional thread messages instead of rewriting the
	// review message itself.
	MsgTriageReviewHeader                       MsgKey = "triage_review_header"
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
	MsgTriageReviewEditSummaryLabel             MsgKey = "triage_review_edit_summary_label"
	MsgTriageReviewEditAssigneeLabel            MsgKey = "triage_review_edit_assignee_label"
	MsgTriageReviewEditFieldsHeader             MsgKey = "triage_review_edit_fields_header"
	MsgTriageReviewEditModalSubmit              MsgKey = "triage_review_edit_modal_submit"
	MsgTriageReviewEditModalClose               MsgKey = "triage_review_edit_modal_close"
	MsgTriageReviewReinvestigateModalTitle      MsgKey = "triage_review_reinvestigate_modal_title"
	MsgTriageReviewReinvestigateInstructionLabel MsgKey = "triage_review_reinvestigate_instruction_label"
	MsgTriageReviewReinvestigateModalSubmit     MsgKey = "triage_review_reinvestigate_modal_submit"
	MsgTriageReviewReinvestigateModalClose      MsgKey = "triage_review_reinvestigate_modal_close"
	MsgTriageReviewFieldRequiredError           MsgKey = "triage_review_field_required_error"
)
