package triage

import "github.com/m-mizutani/gollem"

// triagePlanSchema is the JSON shape the LLM must return on every planner
// turn. It mirrors model.TriagePlan: a discriminated union keyed on `kind`
// where exactly one of `investigate` / `ask` / `complete` is populated.
//
// Used with WithContentType(ContentTypeJSON) + WithResponseSchema so the
// model's structured output lands in agent.Execute's *ExecuteResponse.Texts
// already in the right shape — no tool-calling, no FunctionCall fishing.
func triagePlanSchema() *gollem.Parameter {
	return &gollem.Parameter{
		Title:       "TriagePlan",
		Description: "Decision the planner makes for one triage turn. Set kind to exactly one of investigate / ask / complete and populate the matching payload (the other two payload fields must be omitted).",
		Type:        gollem.TypeObject,
		Properties: map[string]*gollem.Parameter{
			"kind": {
				Type:        gollem.TypeString,
				Description: "Which action this plan represents. Must match the populated payload.",
				Required:    true,
				Enum:        []string{"investigate", "ask", "complete"},
			},
			"message": {
				Type:        gollem.TypeString,
				Description: "Short reporter-facing status message (1-2 sentences).",
				Required:    true,
				MinLength:   intPtr(1),
			},
			"investigate": investigateSchema(),
			"ask":         askSchema(),
			"complete":    completeSchema(),
		},
	}
}

func investigateSchema() *gollem.Parameter {
	return &gollem.Parameter{
		Type:        gollem.TypeObject,
		Description: "Populated when kind=investigate. Schedules subtasks to run in parallel.",
		Properties: map[string]*gollem.Parameter{
			"subtasks": {
				Type:        gollem.TypeArray,
				Description: "Investigation subtasks to run in parallel.",
				MinItems:    intPtr(1),
				Items: &gollem.Parameter{
					Type: gollem.TypeObject,
					Properties: map[string]*gollem.Parameter{
						"id": {
							Type:        gollem.TypeString,
							Description: "Stable identifier for the subtask (used as the child agent session id).",
							Required:    true,
							MinLength:   intPtr(1),
						},
						"request": {
							Type:        gollem.TypeString,
							Description: "Imperative-mood instruction for the child agent (e.g. 'Collect related Slack posts').",
							Required:    true,
							MinLength:   intPtr(1),
						},
						"acceptance_criteria": {
							Type:        gollem.TypeArray,
							Description: "Observable completion conditions. 3-5 bullets describing properties of the output.",
							MinItems:    intPtr(1),
							Items:       &gollem.Parameter{Type: gollem.TypeString},
						},
						"allowed_tools": {
							Type:        gollem.TypeArray,
							Description: "Names of tools the child agent is allowed to call.",
							Items:       &gollem.Parameter{Type: gollem.TypeString},
						},
					},
				},
			},
		},
	}
}

func askSchema() *gollem.Parameter {
	return &gollem.Parameter{
		Type:        gollem.TypeObject,
		Description: "Populated when kind=ask. Asks the reporter follow-up questions through a Slack form.",
		Properties: map[string]*gollem.Parameter{
			"title": {
				Type:        gollem.TypeString,
				Description: "Header text shown above the questions.",
				Required:    true,
				MinLength:   intPtr(1),
			},
			"questions": {
				Type:        gollem.TypeArray,
				Description: "List of independent questions; combine only when answers do not depend on each other.",
				MinItems:    intPtr(1),
				Items: &gollem.Parameter{
					Type: gollem.TypeObject,
					Properties: map[string]*gollem.Parameter{
						"id": {
							Type:        gollem.TypeString,
							Description: "Stable identifier (used as the Slack block_id).",
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
							Description: "Predefined choices. Do not include 'Other' — the form always pairs these with a free-text fallback.",
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
							Description: "True for checkboxes, false for radio. Default false.",
						},
					},
				},
			},
		},
	}
}

func completeSchema() *gollem.Parameter {
	return &gollem.Parameter{
		Type:        gollem.TypeObject,
		Description: "Populated when kind=complete. Concludes triage with a hand-off summary.",
		Properties: map[string]*gollem.Parameter{
			"title": {
				Type:        gollem.TypeString,
				Description: "Short, human-readable ticket title (5-15 words). This is written back to the ticket as ticket.Title and shown as the headline in the Slack hand-off message.",
				Required:    true,
				MinLength:   intPtr(1),
			},
			"summary": {
				Type:        gollem.TypeString,
				Description: "Markdown overview the assignee will see first. This is written back to the ticket as ticket.Description and shown as the body of the Slack hand-off message.",
				Required:    true,
				MinLength:   intPtr(1),
			},
			"assignee": {
				Type:        gollem.TypeObject,
				Description: "Assignment decision. Use kind=unassigned when you cannot confidently pick a single owner.",
				Required:    true,
				Properties: map[string]*gollem.Parameter{
					"kind": {
						Type:        gollem.TypeString,
						Description: "Either 'assigned' or 'unassigned'.",
						Required:    true,
						Enum:        []string{"assigned", "unassigned"},
					},
					"user_id": {
						Type:        gollem.TypeString,
						Description: "Slack user id (e.g. 'U123ABC'). Required when kind=='assigned'; omit when kind=='unassigned'.",
					},
					"reasoning": {
						Type:        gollem.TypeString,
						Description: "Why this person (or why nobody is being assigned).",
						Required:    true,
						MinLength:   intPtr(1),
					},
				},
			},
			"suggested_fields": {
				Type:        gollem.TypeObject,
				Description: "Map of ticket field id -> suggested value.",
				Properties:  map[string]*gollem.Parameter{},
			},
		},
	}
}

func intPtr(v int) *int { return &v }
