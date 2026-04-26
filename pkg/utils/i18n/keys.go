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
)
