package prompt

import (
	_ "embed"
	"strings"
	"text/template"

	"github.com/m-mizutani/goerr/v2"
)

//go:embed system.md
var systemTemplateSource string

//go:embed mention.md
var mentionTemplateSource string

//go:embed triage_plan.md
var triagePlanTemplateSource string

//go:embed triage_subtask.md
var triageSubtaskTemplateSource string

var (
	systemTemplate        = template.Must(template.New("system").Parse(systemTemplateSource))
	mentionTemplate       = template.Must(template.New("mention").Parse(mentionTemplateSource))
	triagePlanTemplate    = template.Must(template.New("triage_plan").Parse(triagePlanTemplateSource))
	triageSubtaskTemplate = template.Must(template.New("triage_subtask").Parse(triageSubtaskTemplateSource))
)

// SystemInput is the data for the system prompt template. It carries the
// static ticket context that does not change between turns inside the same
// thread, and is injected once per agent.Execute call via
// gollem.WithSystemPrompt.
type SystemInput struct {
	Title          string
	Description    string
	InitialMessage string
}

// MentionInput is the data for the per-turn user prompt template. It carries
// only the latest mention, since the prior conversation lives in the gollem
// history layer.
type MentionInput struct {
	MentionAuthor string
	Mention       string
}

// RenderSystem renders the system prompt for the agent.
func RenderSystem(in SystemInput) (string, error) {
	var buf strings.Builder
	if err := systemTemplate.Execute(&buf, in); err != nil {
		return "", goerr.Wrap(err, "failed to execute system template")
	}
	return buf.String(), nil
}

// RenderMention renders the user prompt for the latest mention.
func RenderMention(in MentionInput) (string, error) {
	var buf strings.Builder
	if err := mentionTemplate.Execute(&buf, in); err != nil {
		return "", goerr.Wrap(err, "failed to execute mention template")
	}
	return buf.String(), nil
}

// TriagePlanInput is the data for the triage planner system prompt. It carries
// the static ticket context that does not change between turns. Per-turn
// observations (investigation results, reporter answers) are appended to the
// gollem agent history as user messages instead.
//
// UserGuidance is the workspace-specific additional instruction text the user
// edited via the prompts UI. It is treated as opaque markdown — never parsed
// as a Go template — and embedded into the base template as a separate block
// so that user content starting with a markdown heading does not collide with
// the base template's heading hierarchy.
type TriagePlanInput struct {
	Title          string
	Description    string
	InitialMessage string
	Reporter       string
	UserGuidance   string
	AutoFillFields []AutoFillField
}

// AutoFillField is the per-field briefing the planner sees when a workspace
// has at least one custom field marked auto_fill = true. The struct is the
// projection of FieldDefinition that the prompt template actually needs;
// keeping it here avoids leaking the full domain config type into the
// template and lets the template stay readable.
type AutoFillField struct {
	ID          string
	Name        string
	Type        string
	Description string
	Required    bool
	Options     []AutoFillOption
}

// AutoFillOption is the (id, label, description) tuple the planner is
// allowed to pick from for select / multi-select fields. Description is
// optional but, when present, is rendered in the system prompt so the
// model can disambiguate between similarly named options.
type AutoFillOption struct {
	ID          string
	Label       string
	Description string
}

// TriageSubtaskInput is the data for the triage subtask system prompt. It is
// rendered once per subtask, embedding the planner-specified request and
// acceptance criteria.
type TriageSubtaskInput struct {
	Request            string
	AcceptanceCriteria []string
}

// RenderTriagePlan renders the system prompt for the triage planner agent.
func RenderTriagePlan(in TriagePlanInput) (string, error) {
	var buf strings.Builder
	if err := triagePlanTemplate.Execute(&buf, in); err != nil {
		return "", goerr.Wrap(err, "failed to execute triage_plan template")
	}
	return buf.String(), nil
}

// RenderTriageSubtask renders the system prompt for a triage investigation
// subtask agent.
func RenderTriageSubtask(in TriageSubtaskInput) (string, error) {
	var buf strings.Builder
	if err := triageSubtaskTemplate.Execute(&buf, in); err != nil {
		return "", goerr.Wrap(err, "failed to execute triage_subtask template")
	}
	return buf.String(), nil
}
