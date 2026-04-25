package config_test

import (
	"testing"

	"github.com/m-mizutani/gt"
	"github.com/m-mizutani/shepherd/pkg/domain/model/config"
	"github.com/m-mizutani/shepherd/pkg/domain/types"
)

func validSchema() *config.FieldSchema {
	return &config.FieldSchema{
		Statuses: []config.StatusDef{
			{ID: "open", Name: "Open", Color: "#22c55e", Order: 0},
			{ID: "closed", Name: "Closed", Color: "#6b7280", Order: 1},
		},
		TicketConfig: config.TicketConfig{
			DefaultStatusID: "open",
			ClosedStatusIDs: []string{"closed"},
		},
		Fields: []config.FieldDefinition{
			{
				ID:   "priority",
				Name: "Priority",
				Type: types.FieldTypeSelect,
				Options: []config.FieldOption{
					{ID: "high", Name: "High"},
					{ID: "low", Name: "Low"},
				},
			},
		},
		Labels: config.EntityLabels{
			Ticket:      "Ticket",
			Title:       "Title",
			Description: "Description",
		},
	}
}

func TestFieldSchema_Validate_Valid(t *testing.T) {
	gt.NoError(t, validSchema().Validate())
}

func TestFieldSchema_Validate_NoStatuses(t *testing.T) {
	s := validSchema()
	s.Statuses = nil
	gt.Error(t, s.Validate())
}

func TestFieldSchema_Validate_InvalidStatusID(t *testing.T) {
	s := validSchema()
	s.Statuses[0].ID = "INVALID"
	gt.Error(t, s.Validate())
}

func TestFieldSchema_Validate_DuplicateStatusID(t *testing.T) {
	s := validSchema()
	s.Statuses = append(s.Statuses, config.StatusDef{ID: "open", Name: "Dup"})
	gt.Error(t, s.Validate())
}

func TestFieldSchema_Validate_InvalidDefaultStatus(t *testing.T) {
	s := validSchema()
	s.TicketConfig.DefaultStatusID = "nonexistent"
	gt.Error(t, s.Validate())
}

func TestFieldSchema_Validate_DefaultStatusIsClosed(t *testing.T) {
	s := validSchema()
	s.TicketConfig.DefaultStatusID = "closed"
	gt.Error(t, s.Validate())
}

func TestFieldSchema_Validate_InvalidClosedStatus(t *testing.T) {
	s := validSchema()
	s.TicketConfig.ClosedStatusIDs = []string{"nonexistent"}
	gt.Error(t, s.Validate())
}

func TestFieldSchema_Validate_InvalidFieldID(t *testing.T) {
	s := validSchema()
	s.Fields[0].ID = "INVALID!"
	gt.Error(t, s.Validate())
}

func TestFieldSchema_Validate_DuplicateFieldID(t *testing.T) {
	s := validSchema()
	s.Fields = append(s.Fields, config.FieldDefinition{
		ID:   "priority",
		Name: "Dup Priority",
		Type: types.FieldTypeText,
	})
	gt.Error(t, s.Validate())
}

func TestFieldSchema_Validate_MissingFieldName(t *testing.T) {
	s := validSchema()
	s.Fields[0].Name = ""
	gt.Error(t, s.Validate())
}

func TestFieldSchema_Validate_InvalidFieldType(t *testing.T) {
	s := validSchema()
	s.Fields[0].Type = "invalid-type"
	gt.Error(t, s.Validate())
}

func TestFieldSchema_Validate_SelectWithoutOptions(t *testing.T) {
	s := validSchema()
	s.Fields[0].Options = nil
	gt.Error(t, s.Validate())
}

func TestFieldSchema_Validate_DuplicateOptionID(t *testing.T) {
	s := validSchema()
	s.Fields[0].Options = append(s.Fields[0].Options, config.FieldOption{ID: "high", Name: "Dup"})
	gt.Error(t, s.Validate())
}

func TestFieldSchema_Validate_InvalidOptionID(t *testing.T) {
	s := validSchema()
	s.Fields[0].Options[0].ID = "BAD ID!"
	gt.Error(t, s.Validate())
}

func TestFieldSchema_Validate_TextFieldNoOptionsOK(t *testing.T) {
	s := validSchema()
	s.Fields = []config.FieldDefinition{
		{ID: "note", Name: "Note", Type: types.FieldTypeText},
	}
	gt.NoError(t, s.Validate())
}

func TestFieldSchema_Validate_AllFieldTypes(t *testing.T) {
	for _, ft := range []types.FieldType{
		types.FieldTypeText, types.FieldTypeNumber,
		types.FieldTypeUser, types.FieldTypeDate, types.FieldTypeURL,
	} {
		s := validSchema()
		s.Fields = []config.FieldDefinition{
			{ID: "f", Name: "F", Type: ft},
		}
		gt.NoError(t, s.Validate())
	}
}

func TestFieldSchema_IsClosedStatus(t *testing.T) {
	s := validSchema()
	gt.B(t, s.IsClosedStatus("closed")).True()
	gt.B(t, s.IsClosedStatus("open")).False()
	gt.B(t, s.IsClosedStatus("nonexistent")).False()
}
