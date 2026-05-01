package triage

import (
	"github.com/m-mizutani/gollem"
	domainConfig "github.com/m-mizutani/shepherd/pkg/domain/model/config"
	"github.com/m-mizutani/shepherd/pkg/domain/types"
)

// triagePlanSchema is the JSON shape the LLM must return on every planner
// turn. It mirrors model.TriagePlan: a discriminated union keyed on `kind`
// where exactly one of `probe` / `ask` / `complete` is populated.
//
// autoFill is the subset of the workspace schema's custom fields whose
// AutoFill flag is set; the function uses it to constrain the
// complete.suggested_fields object to typed, enum-bounded properties.
//
// Used with WithContentType(ContentTypeJSON) + WithResponseSchema so the
// model's structured output lands in agent.Execute's *ExecuteResponse.Texts
// already in the right shape — no tool-calling, no FunctionCall fishing.
func triagePlanSchema(autoFill []domainConfig.FieldDefinition) *gollem.Parameter {
	return &gollem.Parameter{
		Title:       "TriagePlan",
		Description: "Decision the planner makes for one triage turn. Set kind to exactly one of probe / ask / complete and populate the matching payload (the other two payload fields must be omitted).",
		Type:        gollem.TypeObject,
		Properties: map[string]*gollem.Parameter{
			"kind": {
				Type:        gollem.TypeString,
				Description: "Which action this plan represents. Must match the populated payload.",
				Required:    true,
				Enum:        []string{"probe", "ask", "complete"},
			},
			"message": {
				Type:        gollem.TypeString,
				Description: "Short reporter-facing status message (1-2 sentences).",
				Required:    true,
				MinLength:   intPtr(1),
			},
			"probe":    probeSchema(),
			"ask":      askSchema(),
			"complete": completeSchema(autoFill),
		},
	}
}

func probeSchema() *gollem.Parameter {
	return &gollem.Parameter{
		Type:        gollem.TypeObject,
		Description: "Populated when kind=probe. Schedules subtasks to run in parallel.",
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

func completeSchema(autoFill []domainConfig.FieldDefinition) *gollem.Parameter {

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
			"description": {
				Type:        gollem.TypeString,
				Description: "Markdown overview the assignee will see first. This is written back to the ticket as ticket.Description and shown as the body of the Slack hand-off message.",
				Required:    true,
				MinLength:   intPtr(1),
			},
			"assignee": {
				Type:        gollem.TypeObject,
				Description: "Assignment decision. Populate user_ids with one or more Slack user ids when you can confidently pick owners; leave user_ids empty (or omit it) to leave the ticket unassigned for the team to pick up.",
				Required:    true,
				Properties: map[string]*gollem.Parameter{
					"user_ids": {
						Type:        gollem.TypeArray,
						Description: "Slack user id strings (e.g. ['U123ABC'] or ['U123ABC', 'U456DEF']). Empty / omitted means unassigned.",
						Items:       &gollem.Parameter{Type: gollem.TypeString},
					},
					"reasoning": {
						Type:        gollem.TypeString,
						Description: "Why these people were picked, or why no confident owner can be chosen.",
						Required:    true,
						MinLength:   intPtr(1),
					},
				},
			},
			"suggested_fields": suggestedFieldsSchema(autoFill),
		},
	}
}

// suggestedFieldsSchema returns the JSON schema constraining the
// complete.suggested_fields object. When autoFill is non-empty, every entry
// becomes a typed property and required entries are listed in Required.
// When autoFill is empty the schema falls back to a free-form object so the
// LLM keeps the ability to volunteer values for non-auto-fill fields.
func suggestedFieldsSchema(autoFill []domainConfig.FieldDefinition) *gollem.Parameter {
	props := make(map[string]*gollem.Parameter, len(autoFill))
	for _, f := range autoFill {
		p := autoFillFieldSchema(f)
		p.Required = f.Required
		props[f.ID] = p
	}
	return &gollem.Parameter{
		Type:        gollem.TypeObject,
		Description: "Map of ticket field id -> suggested value. Auto-fill fields listed in the system prompt MUST appear here with the value shape declared below.",
		Properties:  props,
	}
}

// autoFillFieldSchema renders a single FieldDefinition as the gollem schema
// the LLM must satisfy. select / multi-select are constrained to the
// configured option ids; date is constrained to ISO 8601.
func autoFillFieldSchema(f domainConfig.FieldDefinition) *gollem.Parameter {
	desc := f.Description
	if desc == "" {
		desc = f.Name
	}
	switch f.Type {
	case types.FieldTypeSelect:
		return &gollem.Parameter{
			Type:        gollem.TypeString,
			Description: desc,
			Enum:        optionIDs(f.Options),
		}
	case types.FieldTypeMultiSelect:
		return &gollem.Parameter{
			Type:        gollem.TypeArray,
			Description: desc,
			Items: &gollem.Parameter{
				Type: gollem.TypeString,
				Enum: optionIDs(f.Options),
			},
		}
	case types.FieldTypeNumber:
		return &gollem.Parameter{Type: gollem.TypeNumber, Description: desc}
	case types.FieldTypeDate:
		return &gollem.Parameter{
			Type:        gollem.TypeString,
			Description: desc + " (format YYYY-MM-DD)",
			Pattern:     `^\d{4}-\d{2}-\d{2}$`,
		}
	case types.FieldTypeUser:
		return &gollem.Parameter{Type: gollem.TypeString, Description: desc + " (Slack user id, e.g. U123ABC)"}
	case types.FieldTypeMultiUser:
		return &gollem.Parameter{
			Type:        gollem.TypeArray,
			Description: desc + " (array of Slack user ids)",
			Items:       &gollem.Parameter{Type: gollem.TypeString},
		}
	case types.FieldTypeURL:
		return &gollem.Parameter{Type: gollem.TypeString, Description: desc + " (absolute URL)"}
	default:
		return &gollem.Parameter{Type: gollem.TypeString, Description: desc}
	}
}

func optionIDs(opts []domainConfig.FieldOption) []string {
	out := make([]string, 0, len(opts))
	for _, o := range opts {
		out = append(out, o.ID)
	}
	return out
}

func intPtr(v int) *int { return &v }
