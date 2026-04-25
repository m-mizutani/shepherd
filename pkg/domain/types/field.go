package types

type FieldType string

const (
	FieldTypeText        FieldType = "text"
	FieldTypeNumber      FieldType = "number"
	FieldTypeSelect      FieldType = "select"
	FieldTypeMultiSelect FieldType = "multi-select"
	FieldTypeUser        FieldType = "user"
	FieldTypeMultiUser   FieldType = "multi-user"
	FieldTypeDate        FieldType = "date"
	FieldTypeURL         FieldType = "url"
)

func (ft FieldType) IsValid() bool {
	switch ft {
	case FieldTypeText, FieldTypeNumber, FieldTypeSelect,
		FieldTypeMultiSelect, FieldTypeUser, FieldTypeMultiUser,
		FieldTypeDate, FieldTypeURL:
		return true
	}
	return false
}

func (ft FieldType) String() string { return string(ft) }
