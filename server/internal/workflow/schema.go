package workflow

import (
	"fmt"
)

type FieldType string

const (
	FieldString FieldType = "string"
	FieldNumber FieldType = "number"
	FieldBool   FieldType = "boolean"
	FieldArray  FieldType = "array"
	FieldObject FieldType = "object"
)

type FieldSchema struct {
	Type     FieldType `json:"type"`
	Required bool      `json:"required"`
}

type Schema struct {
	Fields map[string]FieldSchema `json:"fields"`
}

func (s Schema) Validate(value map[string]any) error {
	for name, field := range s.Fields {
		raw, ok := value[name]
		if !ok {
			if field.Required {
				return fmt.Errorf("required field %q is missing", name)
			}
			continue
		}
		if !matchesType(raw, field.Type) {
			return fmt.Errorf("field %q must be %s", name, field.Type)
		}
	}
	return nil
}

func matchesType(value any, fieldType FieldType) bool {
	switch fieldType {
	case FieldString:
		_, ok := value.(string)
		return ok
	case FieldNumber:
		switch value.(type) {
		case int, int64, float64, float32:
			return true
		default:
			return false
		}
	case FieldBool:
		_, ok := value.(bool)
		return ok
	case FieldArray:
		switch value.(type) {
		case []any, []string:
			return true
		default:
			return false
		}
	case FieldObject:
		_, ok := value.(map[string]any)
		return ok
	default:
		return true
	}
}
