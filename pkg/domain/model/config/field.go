package config

import (
	"regexp"
	"slices"

	"github.com/m-mizutani/goerr/v2"
	"github.com/m-mizutani/shepherd/pkg/domain/types"
)

var idPattern = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

var (
	ErrMissingStatuses      = goerr.New("at least one status is required")
	ErrInvalidStatusID      = goerr.New("invalid status ID")
	ErrDuplicateStatusID    = goerr.New("duplicate status ID")
	ErrInvalidDefaultStatus = goerr.New("default_status must reference a defined status")
	ErrClosedStatusDefault  = goerr.New("default_status cannot be a closed status")
	ErrInvalidClosedStatus  = goerr.New("closed_statuses must reference defined statuses")
	ErrInvalidFieldID       = goerr.New("invalid field ID")
	ErrDuplicateFieldID     = goerr.New("duplicate field ID")
	ErrInvalidFieldType     = goerr.New("invalid field type")
	ErrMissingFieldName     = goerr.New("field name is required")
	ErrMissingOptions       = goerr.New("select/multi-select fields require options")
	ErrDuplicateOptionID    = goerr.New("duplicate option ID")
	ErrInvalidOptionID      = goerr.New("invalid option ID")
)

type StatusDef struct {
	ID    string
	Name  string
	Color string
	Order int
}

type TicketConfig struct {
	DefaultStatusID string
	ClosedStatusIDs []string
}

type FieldDefinition struct {
	ID          string
	Name        string
	Type        types.FieldType
	Required    bool
	Description string
	Options     []FieldOption
}

type FieldOption struct {
	ID       string
	Name     string
	Color    string
	Metadata map[string]any
}

type EntityLabels struct {
	Ticket      string
	Title       string
	Description string
}

type FieldSchema struct {
	Statuses     []StatusDef
	TicketConfig TicketConfig
	Fields       []FieldDefinition
	Labels       EntityLabels
}

func (s *FieldSchema) Validate() error {
	if len(s.Statuses) == 0 {
		return goerr.Wrap(ErrMissingStatuses, "at least one status is required")
	}

	statusIDs := make(map[string]bool, len(s.Statuses))
	for _, st := range s.Statuses {
		if !idPattern.MatchString(st.ID) {
			return goerr.Wrap(ErrInvalidStatusID, "status ID must match ^[a-z0-9]+(-[a-z0-9]+)*$",
				goerr.V("status_id", st.ID))
		}
		if statusIDs[st.ID] {
			return goerr.Wrap(ErrDuplicateStatusID, "duplicate status ID",
				goerr.V("status_id", st.ID))
		}
		statusIDs[st.ID] = true
	}

	if s.TicketConfig.DefaultStatusID != "" && !statusIDs[s.TicketConfig.DefaultStatusID] {
		return goerr.Wrap(ErrInvalidDefaultStatus,
			"default_status must reference a defined status",
			goerr.V("status_id", s.TicketConfig.DefaultStatusID))
	}

	for _, cs := range s.TicketConfig.ClosedStatusIDs {
		if !statusIDs[cs] {
			return goerr.Wrap(ErrInvalidClosedStatus,
				"closed_statuses must reference defined statuses",
				goerr.V("status_id", cs))
		}
	}

	if s.TicketConfig.DefaultStatusID != "" &&
		slices.Contains(s.TicketConfig.ClosedStatusIDs, s.TicketConfig.DefaultStatusID) {
		return goerr.Wrap(ErrClosedStatusDefault,
			"default_status cannot be a closed status",
			goerr.V("status_id", s.TicketConfig.DefaultStatusID))
	}

	fieldIDs := make(map[string]bool, len(s.Fields))
	for i, f := range s.Fields {
		if err := validateFieldDefinition(&f); err != nil {
			return goerr.Wrap(err, "invalid field",
				goerr.V("field_index", i))
		}
		if fieldIDs[f.ID] {
			return goerr.Wrap(ErrDuplicateFieldID, "duplicate field ID",
				goerr.V("field_id", f.ID))
		}
		fieldIDs[f.ID] = true
	}

	return nil
}

func validateFieldDefinition(f *FieldDefinition) error {
	if !idPattern.MatchString(f.ID) {
		return goerr.Wrap(ErrInvalidFieldID, "field ID must match ^[a-z0-9]+(-[a-z0-9]+)*$",
			goerr.V("field_id", f.ID))
	}
	if f.Name == "" {
		return goerr.Wrap(ErrMissingFieldName, "field name is required",
			goerr.V("field_id", f.ID))
	}
	if !f.Type.IsValid() {
		return goerr.Wrap(ErrInvalidFieldType, "invalid field type",
			goerr.V("field_id", f.ID), goerr.V("field_type", string(f.Type)))
	}

	if f.Type == types.FieldTypeSelect || f.Type == types.FieldTypeMultiSelect {
		if len(f.Options) == 0 {
			return goerr.Wrap(ErrMissingOptions, "select/multi-select fields require options",
				goerr.V("field_id", f.ID))
		}
		optIDs := make(map[string]bool, len(f.Options))
		for _, opt := range f.Options {
			if !idPattern.MatchString(opt.ID) {
				return goerr.Wrap(ErrInvalidOptionID, "option ID must match pattern",
					goerr.V("field_id", f.ID), goerr.V("option_id", opt.ID))
			}
			if optIDs[opt.ID] {
				return goerr.Wrap(ErrDuplicateOptionID, "duplicate option ID",
					goerr.V("field_id", f.ID), goerr.V("option_id", opt.ID))
			}
			optIDs[opt.ID] = true
		}
	}

	return nil
}

func (s *FieldSchema) IsClosedStatus(statusID string) bool {
	for _, id := range s.TicketConfig.ClosedStatusIDs {
		if id == statusID {
			return true
		}
	}
	return false
}
