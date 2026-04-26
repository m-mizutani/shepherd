package triage

import (
	"context"

	"github.com/m-mizutani/goerr/v2"
	"github.com/m-mizutani/gollem"
)

// proposeTools returns the three propose_* tool specs the LLM is allowed to
// emit during a planning turn. The tools exist purely to advertise schemas
// to the model; their Run is never invoked because llmPlan calls
// Session.Generate directly (no agent loop) and decodes the FunctionCall
// from the response.
func proposeTools() []gollem.Tool {
	return []gollem.Tool{
		&proposeInvestigateTool{},
		&proposeAskTool{},
		&proposeCompleteTool{},
	}
}

// commonMessageParam is shared across all propose_* tools so the LLM is
// forced to attach a short reporter-facing status message to every plan.
func commonMessageParam() *gollem.Parameter {
	return &gollem.Parameter{
		Type:        gollem.TypeString,
		Description: "Short, reporter-facing status message (1-2 sentences) explaining the current direction. Required.",
		Required:    true,
		MinLength:   intPtr(1),
	}
}

func intPtr(v int) *int { return &v }

// errToolNotInvokable is returned by every propose_* tool's Run because the
// triage planner never executes propose_* tools — it only inspects the
// FunctionCall the model emitted. Hitting Run means an Agent loop is being
// driven over these tools by mistake; surfacing an error makes that obvious.
var errToolNotInvokable = goerr.New("propose_* tools are spec-only and must not be executed by an agent loop")

// --- propose_investigate ---

type proposeInvestigateTool struct{}

func (t *proposeInvestigateTool) Spec() gollem.ToolSpec {
	return gollem.ToolSpec{
		Name: proposeInvestigateToolName,
		Description: "Schedule one or more investigation subtasks to be run in parallel by child agents. " +
			"Use this when more information is needed before deciding what to ask the reporter or how to complete triage.",
		Parameters: map[string]*gollem.Parameter{
			"message": commonMessageParam(),
			"subtasks": {
				Type:        gollem.TypeArray,
				Description: "Investigation subtasks to run in parallel.",
				Required:    true,
				MinItems:    intPtr(1),
				Items: &gollem.Parameter{
					Type: gollem.TypeObject,
					Properties: map[string]*gollem.Parameter{
						"id": {
							Type:        gollem.TypeString,
							Description: "Stable identifier for the subtask. Used as the child agent session id.",
							Required:    true,
							MinLength:   intPtr(1),
						},
						"request": {
							Type:        gollem.TypeString,
							Description: "Imperative-mood instruction for the child agent (e.g. 'Collect related Slack posts', 'Identify the affected service').",
							Required:    true,
							MinLength:   intPtr(1),
						},
						"acceptance_criteria": {
							Type:        gollem.TypeArray,
							Description: "Observable completion conditions (3-5 bullets). Each item should describe an observable property of the output (e.g. 'returns at least N items or explicitly states none').",
							Required:    true,
							MinItems:    intPtr(1),
							Items:       &gollem.Parameter{Type: gollem.TypeString},
						},
						"allowed_tools": {
							Type:        gollem.TypeArray,
							Description: "Names of tools the child agent is allowed to call. Restrict to tools relevant to this subtask.",
							Required:    true,
							Items:       &gollem.Parameter{Type: gollem.TypeString},
						},
					},
				},
			},
		},
	}
}

func (t *proposeInvestigateTool) Run(_ context.Context, _ map[string]any) (map[string]any, error) {
	return nil, errToolNotInvokable
}

// --- propose_ask ---

type proposeAskTool struct{}

func (t *proposeAskTool) Spec() gollem.ToolSpec {
	return gollem.ToolSpec{
		Name: proposeAskToolName,
		Description: "Ask the reporter follow-up questions through a Slack message form. " +
			"Use this when the missing information cannot be derived from investigation alone and must come from the reporter.",
		Parameters: map[string]*gollem.Parameter{
			"message": commonMessageParam(),
			"title": {
				Type:        gollem.TypeString,
				Description: "Header text shown above the questions in the Slack message.",
				Required:    true,
				MinLength:   intPtr(1),
			},
			"questions": {
				Type:        gollem.TypeArray,
				Description: "List of questions. Independent questions may be combined into one Ask; if the answer to one would change the next, defer the dependent question to a later iteration.",
				Required:    true,
				MinItems:    intPtr(1),
				Items: &gollem.Parameter{
					Type: gollem.TypeObject,
					Properties: map[string]*gollem.Parameter{
						"id": {
							Type:        gollem.TypeString,
							Description: "Stable identifier for the question. Used as the Slack block_id, so it must be unique within this Ask.",
							Required:    true,
							MinLength:   intPtr(1),
						},
						"label": {
							Type:        gollem.TypeString,
							Description: "Question text shown to the reporter.",
							Required:    true,
							MinLength:   intPtr(1),
						},
						"help": {
							Type:        gollem.TypeString,
							Description: "Optional supplementary explanation displayed under the label.",
						},
						"choices": {
							Type:        gollem.TypeArray,
							Description: "Predefined choices. The form always pairs these with a free-text 'other' input, so do not include 'Other' as a choice here.",
							Required:    true,
							MinItems:    intPtr(1),
							Items: &gollem.Parameter{
								Type: gollem.TypeObject,
								Properties: map[string]*gollem.Parameter{
									"id":    {Type: gollem.TypeString, Required: true, MinLength: intPtr(1)},
									"label": {Type: gollem.TypeString, Required: true, MinLength: intPtr(1)},
								},
							},
						},
						"multiple": {
							Type:        gollem.TypeBoolean,
							Description: "True for checkboxes (multi-select), false for radio (single-select). Default false.",
						},
					},
				},
			},
		},
	}
}

func (t *proposeAskTool) Run(_ context.Context, _ map[string]any) (map[string]any, error) {
	return nil, errToolNotInvokable
}

// --- propose_complete ---

type proposeCompleteTool struct{}

func (t *proposeCompleteTool) Spec() gollem.ToolSpec {
	return gollem.ToolSpec{
		Name: proposeCompleteToolName,
		Description: "Conclude triage and produce the hand-off summary for the assignee. " +
			"Choose this only when no further investigation or questions are needed.",
		Parameters: map[string]*gollem.Parameter{
			"message": commonMessageParam(),
			"summary": {
				Type:        gollem.TypeString,
				Description: "Markdown overview the assignee will see first.",
				Required:    true,
				MinLength:   intPtr(1),
			},
			"assignee": {
				Type:        gollem.TypeObject,
				Description: "Assignment decision. Use 'unassigned' when you cannot confidently pick a single owner; do not invent one.",
				Required:    true,
				Properties: map[string]*gollem.Parameter{
					"kind": {
						Type:        gollem.TypeString,
						Description: "Either 'assigned' (you are confident) or 'unassigned' (let the team decide).",
						Required:    true,
						Enum:        []string{"assigned", "unassigned"},
					},
					"user_id": {
						Type:        gollem.TypeString,
						Description: "Slack user id for the assignee (e.g. 'U123ABC'). Required when kind=='assigned'; must be omitted when kind=='unassigned'.",
					},
					"reasoning": {
						Type:        gollem.TypeString,
						Description: "Why this person (or why nobody is being assigned). Required in both cases.",
						Required:    true,
						MinLength:   intPtr(1),
					},
				},
			},
			"key_findings": {
				Type:        gollem.TypeArray,
				Description: "Concise bullet points the assignee should read first.",
				Items:       &gollem.Parameter{Type: gollem.TypeString},
			},
			"next_steps": {
				Type:        gollem.TypeArray,
				Description: "Recommended actions for the assignee (e.g. 'check log retention', 'contact security team').",
				Items:       &gollem.Parameter{Type: gollem.TypeString},
			},
			"similar_tickets": {
				Type:        gollem.TypeArray,
				Description: "Ticket IDs of related past tickets discovered during investigation.",
				Items:       &gollem.Parameter{Type: gollem.TypeString},
			},
			"answer_summary": {
				Type:        gollem.TypeObject,
				Description: "Map of question label -> reporter answer summary. Used by the assignee to recall the reporter's clarifications. Free-form keys.",
				Properties:  map[string]*gollem.Parameter{},
			},
			"suggested_fields": {
				Type:        gollem.TypeObject,
				Description: "Map of ticket field id -> suggested value. Used to seed structured ticket fields. Free-form keys.",
				Properties:  map[string]*gollem.Parameter{},
			},
		},
	}
}

func (t *proposeCompleteTool) Run(_ context.Context, _ map[string]any) (map[string]any, error) {
	return nil, errToolNotInvokable
}
