package config_test

import (
	"testing"

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
	if err := validSchema().Validate(); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestFieldSchema_Validate_NoStatuses(t *testing.T) {
	s := validSchema()
	s.Statuses = nil
	if err := s.Validate(); err == nil {
		t.Fatal("expected error for missing statuses")
	}
}

func TestFieldSchema_Validate_InvalidStatusID(t *testing.T) {
	s := validSchema()
	s.Statuses[0].ID = "INVALID"
	if err := s.Validate(); err == nil {
		t.Fatal("expected error for invalid status ID")
	}
}

func TestFieldSchema_Validate_DuplicateStatusID(t *testing.T) {
	s := validSchema()
	s.Statuses = append(s.Statuses, config.StatusDef{ID: "open", Name: "Dup"})
	if err := s.Validate(); err == nil {
		t.Fatal("expected error for duplicate status ID")
	}
}

func TestFieldSchema_Validate_InvalidDefaultStatus(t *testing.T) {
	s := validSchema()
	s.TicketConfig.DefaultStatusID = "nonexistent"
	if err := s.Validate(); err == nil {
		t.Fatal("expected error for invalid default status")
	}
}

func TestFieldSchema_Validate_DefaultStatusIsClosed(t *testing.T) {
	s := validSchema()
	s.TicketConfig.DefaultStatusID = "closed"
	if err := s.Validate(); err == nil {
		t.Fatal("expected error when default status is a closed status")
	}
}

func TestFieldSchema_Validate_InvalidClosedStatus(t *testing.T) {
	s := validSchema()
	s.TicketConfig.ClosedStatusIDs = []string{"nonexistent"}
	if err := s.Validate(); err == nil {
		t.Fatal("expected error for invalid closed status reference")
	}
}

func TestFieldSchema_Validate_InvalidFieldID(t *testing.T) {
	s := validSchema()
	s.Fields[0].ID = "INVALID!"
	if err := s.Validate(); err == nil {
		t.Fatal("expected error for invalid field ID")
	}
}

func TestFieldSchema_Validate_DuplicateFieldID(t *testing.T) {
	s := validSchema()
	s.Fields = append(s.Fields, config.FieldDefinition{
		ID:   "priority",
		Name: "Dup Priority",
		Type: types.FieldTypeText,
	})
	if err := s.Validate(); err == nil {
		t.Fatal("expected error for duplicate field ID")
	}
}

func TestFieldSchema_Validate_MissingFieldName(t *testing.T) {
	s := validSchema()
	s.Fields[0].Name = ""
	if err := s.Validate(); err == nil {
		t.Fatal("expected error for missing field name")
	}
}

func TestFieldSchema_Validate_InvalidFieldType(t *testing.T) {
	s := validSchema()
	s.Fields[0].Type = "invalid-type"
	if err := s.Validate(); err == nil {
		t.Fatal("expected error for invalid field type")
	}
}

func TestFieldSchema_Validate_SelectWithoutOptions(t *testing.T) {
	s := validSchema()
	s.Fields[0].Options = nil
	if err := s.Validate(); err == nil {
		t.Fatal("expected error for select field without options")
	}
}

func TestFieldSchema_Validate_DuplicateOptionID(t *testing.T) {
	s := validSchema()
	s.Fields[0].Options = append(s.Fields[0].Options, config.FieldOption{ID: "high", Name: "Dup"})
	if err := s.Validate(); err == nil {
		t.Fatal("expected error for duplicate option ID")
	}
}

func TestFieldSchema_Validate_InvalidOptionID(t *testing.T) {
	s := validSchema()
	s.Fields[0].Options[0].ID = "BAD ID!"
	if err := s.Validate(); err == nil {
		t.Fatal("expected error for invalid option ID")
	}
}

func TestFieldSchema_Validate_TextFieldNoOptionsOK(t *testing.T) {
	s := validSchema()
	s.Fields = []config.FieldDefinition{
		{ID: "note", Name: "Note", Type: types.FieldTypeText},
	}
	if err := s.Validate(); err != nil {
		t.Fatalf("expected no error for text field without options, got %v", err)
	}
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
		if err := s.Validate(); err != nil {
			t.Errorf("expected no error for field type %q, got %v", ft, err)
		}
	}
}

func TestFieldSchema_IsClosedStatus(t *testing.T) {
	s := validSchema()
	if !s.IsClosedStatus("closed") {
		t.Error("expected 'closed' to be a closed status")
	}
	if s.IsClosedStatus("open") {
		t.Error("expected 'open' to not be a closed status")
	}
	if s.IsClosedStatus("nonexistent") {
		t.Error("expected 'nonexistent' to not be a closed status")
	}
}
