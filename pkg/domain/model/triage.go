package model

import (
	"github.com/m-mizutani/goerr/v2"
	"github.com/m-mizutani/shepherd/pkg/domain/types"
)

// TriagePlan is the decoded LLM response for a single planning turn. The LLM
// chooses exactly one of probe / ask / complete, and the corresponding
// payload field (Probe / Ask / Complete) is set while the others remain nil.
//
// The plan itself is not persisted; the canonical record of past plans lives
// in the gollem agent history (session "{wsID}/{ticketID}/plan"). Use
// LoadLatestTriagePlan to recover the most recent plan from that history.
type TriagePlan struct {
	Kind     types.PlanKind `json:"kind"`
	Message  string         `json:"message"` // Reporter-facing short status update; required for every plan.
	Probe    *Probe         `json:"probe,omitempty"`
	Ask      *Ask           `json:"ask,omitempty"`
	Complete *Complete      `json:"complete,omitempty"`
}

// Validate ensures Kind matches the populated payload and that Message is set.
func (p *TriagePlan) Validate() error {
	if p == nil {
		return goerr.New("triage plan is nil")
	}
	if p.Message == "" {
		return goerr.New("triage plan message is empty")
	}
	switch p.Kind {
	case types.PlanProbe:
		if p.Probe == nil {
			return goerr.New("probe payload missing", goerr.V("kind", p.Kind))
		}
		if p.Ask != nil || p.Complete != nil {
			return goerr.New("only probe payload should be set", goerr.V("kind", p.Kind))
		}
		return p.Probe.Validate()
	case types.PlanAsk:
		if p.Ask == nil {
			return goerr.New("ask payload missing", goerr.V("kind", p.Kind))
		}
		if p.Probe != nil || p.Complete != nil {
			return goerr.New("only ask payload should be set", goerr.V("kind", p.Kind))
		}
		return p.Ask.Validate()
	case types.PlanComplete:
		if p.Complete == nil {
			return goerr.New("complete payload missing", goerr.V("kind", p.Kind))
		}
		if p.Probe != nil || p.Ask != nil {
			return goerr.New("only complete payload should be set", goerr.V("kind", p.Kind))
		}
		return p.Complete.Validate()
	default:
		return goerr.New("unknown plan kind", goerr.V("kind", p.Kind))
	}
}

// Probe launches one or more child investigation subtasks in parallel.
type Probe struct {
	Subtasks []Subtask `json:"subtasks"`
}

func (p *Probe) Validate() error {
	if len(p.Subtasks) == 0 {
		return goerr.New("probe must have at least one subtask")
	}
	seen := make(map[types.SubtaskID]struct{}, len(p.Subtasks))
	for idx, st := range p.Subtasks {
		if err := st.Validate(); err != nil {
			return goerr.Wrap(err, "invalid subtask", goerr.V("index", idx))
		}
		if _, dup := seen[st.ID]; dup {
			return goerr.New("duplicate subtask id", goerr.V("id", st.ID))
		}
		seen[st.ID] = struct{}{}
	}
	return nil
}

// Subtask describes a single piece of investigation delegated to a child agent.
type Subtask struct {
	ID                 types.SubtaskID `json:"id"`
	Request            string          `json:"request"`             // Imperative-mood instruction (e.g. "Collect ...").
	AcceptanceCriteria []string        `json:"acceptance_criteria"` // Observable completion conditions, 3-5 items expected.
	AllowedTools       []string        `json:"allowed_tools"`       // Tool names the child agent is allowed to invoke.
}

func (s *Subtask) Validate() error {
	if s.ID == "" {
		return goerr.New("subtask id is empty")
	}
	if s.Request == "" {
		return goerr.New("subtask request is empty", goerr.V("id", s.ID))
	}
	if len(s.AcceptanceCriteria) == 0 {
		return goerr.New("subtask must have at least one acceptance criterion", goerr.V("id", s.ID))
	}
	return nil
}

// Ask presents one or more questions to the reporter via a Slack message with
// inline input blocks. The Submit click sends back a block_actions payload
// containing all input values keyed by the original Question.ID.
type Ask struct {
	Title     string     `json:"title"`
	Questions []Question `json:"questions"`
}

func (a *Ask) Validate() error {
	if len(a.Questions) == 0 {
		return goerr.New("ask must have at least one question")
	}
	seen := make(map[types.QuestionID]struct{}, len(a.Questions))
	for idx, q := range a.Questions {
		if err := q.Validate(); err != nil {
			return goerr.Wrap(err, "invalid question", goerr.V("index", idx))
		}
		if _, dup := seen[q.ID]; dup {
			return goerr.New("duplicate question id", goerr.V("id", q.ID))
		}
		seen[q.ID] = struct{}{}
	}
	return nil
}

// Question is a single prompt to the reporter. It always carries pre-defined
// choices plus an implicit free-text fallback ("when none of the above
// applies").
type Question struct {
	ID       types.QuestionID `json:"id"`
	Label    string           `json:"label"`
	Help     string           `json:"help,omitempty"`
	Choices  []Choice         `json:"choices"`
	Multiple bool             `json:"multiple,omitempty"` // false: radio buttons, true: checkboxes.
}

func (q *Question) Validate() error {
	if q.ID == "" {
		return goerr.New("question id is empty")
	}
	if q.Label == "" {
		return goerr.New("question label is empty", goerr.V("id", q.ID))
	}
	if len(q.Choices) == 0 {
		return goerr.New("question must have at least one choice", goerr.V("id", q.ID))
	}
	seen := make(map[types.ChoiceID]struct{}, len(q.Choices))
	for _, c := range q.Choices {
		if err := c.Validate(); err != nil {
			return goerr.Wrap(err, "invalid choice", goerr.V("question", q.ID))
		}
		if _, dup := seen[c.ID]; dup {
			return goerr.New("duplicate choice id", goerr.V("question", q.ID), goerr.V("choice", c.ID))
		}
		seen[c.ID] = struct{}{}
	}
	return nil
}

type Choice struct {
	ID    types.ChoiceID `json:"id"`
	Label string         `json:"label"`
}

func (c *Choice) Validate() error {
	if c.ID == "" {
		return goerr.New("choice id is empty")
	}
	if c.Label == "" {
		return goerr.New("choice label is empty", goerr.V("id", c.ID))
	}
	return nil
}

// Answer is the reporter's response to a single Question. SelectedIDs lists
// chosen choices (one for radio, multiple for checkboxes); OtherText carries
// the free-text fallback. At least one of the two must be non-empty.
type Answer struct {
	QuestionID  types.QuestionID `json:"question_id"`
	SelectedIDs []types.ChoiceID `json:"selected_ids,omitempty"`
	OtherText   string           `json:"other_text,omitempty"`
}

// IsValid reports whether the reporter actually answered the question.
func (a *Answer) IsValid() bool {
	if len(a.SelectedIDs) > 0 {
		return true
	}
	for _, r := range a.OtherText {
		if r != ' ' && r != '\t' && r != '\n' && r != '\r' {
			return true
		}
	}
	return false
}

// Complete carries the LLM's final triage summary for the assignee.
//
// Title and Description are written back to the ticket as ticket.Title and
// ticket.Description respectively when triage finalises (or when a human
// confirms the proposal in the review modal). Existing tests / data may
// still carry plans without Title; Validate accepts an empty Title for
// backwards compatibility, in which case finalize leaves ticket.Title alone.
type Complete struct {
	Assignee        AssigneeDecision  `json:"assignee"`
	SuggestedFields map[string]any    `json:"suggested_fields,omitempty"`
	SimilarTickets  []types.TicketID  `json:"similar_tickets,omitempty"`
	KeyFindings     []string          `json:"key_findings,omitempty"`
	AnswerSummary   map[string]string `json:"answer_summary,omitempty"`
	Title           string            `json:"title,omitempty"`
	Description     string            `json:"description"`
	NextSteps       []string          `json:"next_steps,omitempty"`
}

func (c *Complete) Validate() error {
	if c.Description == "" {
		return goerr.New("complete description is empty")
	}
	return c.Assignee.Validate()
}

// AssigneeDecision captures whether the LLM picked one or more assignees or
// intentionally left the ticket unassigned. Reasoning is required in either
// case; an empty UserIDs list means "unassigned" and a non-empty list means
// "assigned" — the same information `kind` previously encoded redundantly.
type AssigneeDecision struct {
	UserIDs   []types.SlackUserID `json:"user_ids,omitempty"`
	Reasoning string              `json:"reasoning"`
}

// Assigned reports whether the decision picks at least one concrete owner.
// Unassigned decisions are simply the absence of any user id.
func (d *AssigneeDecision) Assigned() bool {
	return len(d.UserIDs) > 0
}

func (d *AssigneeDecision) Validate() error {
	if d.Reasoning == "" {
		return goerr.New("assignee reasoning is empty")
	}
	for _, id := range d.UserIDs {
		if id == "" {
			return goerr.New("assignee decision contains empty user id")
		}
	}
	return nil
}
