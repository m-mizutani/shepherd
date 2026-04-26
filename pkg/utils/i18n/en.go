package i18n

var en = map[MsgKey]string{
	MsgTicketCreated:     "<{url}|Ticket #{id}> created",
	MsgStatusChange:      "Status: *{old}* → *{new}*",
	MsgStatusChangeLabel: "Status",

	MsgTriageProgressHeader:  "Triage in progress: {message}",
	MsgTriageProgressQueued:  "⏳ {request} — queued",
	MsgTriageProgressRunning: "🔄 {request} — {trace}",
	MsgTriageProgressDone:    "✅ {request} — done",
	MsgTriageProgressFailed:  "❌ {request} — failed: {error}",

	MsgTriageAskHeader:          "Please fill in the information needed for triage:\n_{message}_",
	MsgTriageAskSubmitButton:    "Submit",
	MsgTriageAskOtherTextLabel:  "If none of the above applies (free-form)",
	MsgTriageAskReceived:        "Thanks, your answers were received.",
	MsgTriageAskInvalidated:     "This form is no longer valid.",
	MsgTriageAskValidationError: "Please choose at least one option or fill in the free-form field.",
	MsgTriageAskHistoryMissing:  "Form context is missing; cannot accept this submission.",

	MsgTriageCompleteHeaderAssigned:   "Triage completed",
	MsgTriageCompleteHeaderUnassigned: "Triage completed (no assignee)",
	MsgTriageCompleteAssigneeMention:  "<@{user}>, please take this over.",
	MsgTriageCompleteSectionSummary:   "*Summary*",
	MsgTriageCompleteSectionFindings:  "*Key findings*",
	MsgTriageCompleteSectionAnswers:   "*Reporter answers*",
	MsgTriageCompleteSectionSimilar:   "*Similar tickets*",
	MsgTriageCompleteSectionNextSteps: "*Recommended next steps*",
	MsgTriageCompleteUnassignedReason: "Reason: {reason}",
	MsgTriageAbortedHeader:            "Triage could not be completed",
	MsgTriageAbortedReason:            "Reason: {reason}",
}
