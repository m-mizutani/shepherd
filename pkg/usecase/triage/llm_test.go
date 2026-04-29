package triage_test

import (
	"testing"

	"github.com/m-mizutani/gt"
	"github.com/m-mizutani/shepherd/pkg/domain/model"
	domainConfig "github.com/m-mizutani/shepherd/pkg/domain/model/config"
	"github.com/m-mizutani/shepherd/pkg/domain/types"
	"github.com/m-mizutani/shepherd/pkg/usecase/triage"
)

func severityField(required bool) domainConfig.FieldDefinition {
	return domainConfig.FieldDefinition{
		ID:       "severity",
		Name:     "Severity",
		Type:     types.FieldTypeSelect,
		Required: required,
		AutoFill: true,
		Options: []domainConfig.FieldOption{
			{ID: "p0", Name: "Sev 0"},
			{ID: "p1", Name: "Sev 1"},
		},
	}
}

func tagsField() domainConfig.FieldDefinition {
	return domainConfig.FieldDefinition{
		ID:       "tags",
		Name:     "Tags",
		Type:     types.FieldTypeMultiSelect,
		AutoFill: true,
		Options: []domainConfig.FieldOption{
			{ID: "frontend", Name: "Frontend"},
			{ID: "backend", Name: "Backend"},
		},
	}
}

func completePlan(values map[string]any) *model.TriagePlan {
	uid := types.SlackUserID("U123")
	return &model.TriagePlan{
		Kind:    types.PlanComplete,
		Message: "done",
		Complete: &model.Complete{
			Summary: "ok",
			Assignee: model.AssigneeDecision{
				Kind:      types.AssigneeAssigned,
				UserID:    &uid,
				Reasoning: "owner",
			},
			SuggestedFields: values,
		},
	}
}

func TestValidatePlanAutoFill_AcceptsValidValues(t *testing.T) {
	plan := completePlan(map[string]any{
		"severity": "p0",
		"tags":     []any{"frontend", "backend"},
	})
	err := triage.ValidatePlanAutoFillForTest(plan, []domainConfig.FieldDefinition{
		severityField(true), tagsField(),
	})
	gt.NoError(t, err)
}

func TestValidatePlanAutoFill_RequiredFieldMissing(t *testing.T) {
	plan := completePlan(map[string]any{
		"tags": []any{"frontend"},
	})
	err := triage.ValidatePlanAutoFillForTest(plan, []domainConfig.FieldDefinition{
		severityField(true), tagsField(),
	})
	gt.Error(t, err)
}

func TestValidatePlanAutoFill_OptionalFieldMissingIsOK(t *testing.T) {
	plan := completePlan(map[string]any{
		"severity": "p1",
	})
	err := triage.ValidatePlanAutoFillForTest(plan, []domainConfig.FieldDefinition{
		severityField(true), tagsField(),
	})
	gt.NoError(t, err)
}

func TestValidatePlanAutoFill_SelectValueNotInOptions(t *testing.T) {
	plan := completePlan(map[string]any{
		"severity": "made-up",
	})
	err := triage.ValidatePlanAutoFillForTest(plan, []domainConfig.FieldDefinition{
		severityField(true),
	})
	gt.Error(t, err)
}

func TestValidatePlanAutoFill_MultiSelectValueNotInOptions(t *testing.T) {
	plan := completePlan(map[string]any{
		"tags": []any{"frontend", "weird-tag"},
	})
	err := triage.ValidatePlanAutoFillForTest(plan, []domainConfig.FieldDefinition{
		tagsField(),
	})
	gt.Error(t, err)
}

func TestValidatePlanAutoFill_TypeMismatchOnNumber(t *testing.T) {
	field := domainConfig.FieldDefinition{
		ID:       "count",
		Name:     "Count",
		Type:     types.FieldTypeNumber,
		AutoFill: true,
		Required: true,
	}
	plan := completePlan(map[string]any{"count": "not-a-number"})
	err := triage.ValidatePlanAutoFillForTest(plan, []domainConfig.FieldDefinition{field})
	gt.Error(t, err)
}

func TestValidatePlanAutoFill_DateFormat(t *testing.T) {
	field := domainConfig.FieldDefinition{
		ID:       "due",
		Name:     "Due",
		Type:     types.FieldTypeDate,
		AutoFill: true,
		Required: true,
	}
	good := completePlan(map[string]any{"due": "2026-04-29"})
	gt.NoError(t, triage.ValidatePlanAutoFillForTest(good, []domainConfig.FieldDefinition{field}))

	bad := completePlan(map[string]any{"due": "29/04/2026"})
	gt.Error(t, triage.ValidatePlanAutoFillForTest(bad, []domainConfig.FieldDefinition{field}))
}

func TestValidatePlanAutoFill_NonCompletePlanShortCircuits(t *testing.T) {
	plan := &model.TriagePlan{
		Kind:    types.PlanAsk,
		Message: "asking",
		Ask: &model.Ask{
			Title: "Q?",
			Questions: []model.Question{{
				ID:    "q1",
				Label: "What?",
				Choices: []model.Choice{
					{ID: "c1", Label: "X"},
				},
			}},
		},
	}
	err := triage.ValidatePlanAutoFillForTest(plan, []domainConfig.FieldDefinition{severityField(true)})
	gt.NoError(t, err)
}
