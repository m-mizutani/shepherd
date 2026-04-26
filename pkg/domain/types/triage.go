package types

// SubtaskID identifies a single investigation subtask within a triage iteration.
type SubtaskID string

func (id SubtaskID) String() string { return string(id) }

// QuestionID identifies a question presented to the reporter as part of an Ask plan.
// It is also used as the Slack block_id when rendering the question form, so that
// the submission payload can be matched back to the question definition.
type QuestionID string

func (id QuestionID) String() string { return string(id) }

// ChoiceID identifies a single selectable option within a Question.
type ChoiceID string

func (id ChoiceID) String() string { return string(id) }

// PlanKind discriminates which action the LLM proposed for the current
// iteration. Exactly one of TriagePlan.Investigate / Ask / Complete must be
// non-nil based on this Kind.
type PlanKind string

const (
	PlanInvestigate PlanKind = "investigate"
	PlanAsk         PlanKind = "ask"
	PlanComplete    PlanKind = "complete"
)

// AssigneeDecisionKind expresses whether the LLM decided to assign someone or
// intentionally left the ticket unassigned.
type AssigneeDecisionKind string

const (
	AssigneeAssigned   AssigneeDecisionKind = "assigned"
	AssigneeUnassigned AssigneeDecisionKind = "unassigned"
)
