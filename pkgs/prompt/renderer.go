package prompt

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// Renderer handles template rendering with variable substitution
type Renderer struct{}

// NewRenderer creates a new template renderer
func NewRenderer() *Renderer {
	return &Renderer{}
}

// Render renders a template with the given context
func (r *Renderer) Render(template *Template, ctx *RenderContext) (string, error) {
	// Validate and apply defaults
	if err := r.validateAndApplyDefaults(template, ctx); err != nil {
		return "", err
	}

	// Replace variables in content
	result := template.Content

	for name, value := range ctx.Values {
		// Convert value to string based on type
		strValue, err := r.valueToString(value)
		if err != nil {
			return "", fmt.Errorf("failed to convert variable %s: %w", name, err)
		}

		// Replace all occurrences of {{ name }}
		placeholder := fmt.Sprintf("{{ %s }}", name)
		result = strings.ReplaceAll(result, placeholder, strValue)

		// Also handle no-space version {{name}}
		placeholder = fmt.Sprintf("{{%s}}", name)
		result = strings.ReplaceAll(result, placeholder, strValue)
	}

	return result, nil
}

// validateAndApplyDefaults validates the context and applies default values
func (r *Renderer) validateAndApplyDefaults(template *Template, ctx *RenderContext) error {
	if ctx.Values == nil {
		ctx.Values = make(map[string]interface{})
	}

	var errors ValidationErrors

	// Check all variables in the template
	for name, varDef := range template.Variables {
		value, provided := ctx.Values[name]

		// If not provided, check if required or has default
		if !provided {
			if varDef.Required && varDef.Default == "" {
				errors = append(errors, ValidationError{
					Field:   name,
					Message: "required variable not provided",
				})
				continue
			}

			// Apply default value
			if varDef.Default != "" {
				defaultValue, err := r.parseValue(varDef.Default, varDef.Type)
				if err != nil {
					errors = append(errors, ValidationError{
						Field:   name,
						Message: fmt.Sprintf("failed to parse default value: %v", err),
					})
					continue
				}
				ctx.Values[name] = defaultValue
			}
		} else {
			// Validate type if provided
			if err := r.validateType(value, varDef.Type); err != nil {
				errors = append(errors, ValidationError{
					Field:   name,
					Message: fmt.Sprintf("invalid type: %v", err),
				})
			}
		}
	}

	if len(errors) > 0 {
		return errors
	}

	return nil
}

// parseValue parses a string value into the appropriate type
func (r *Renderer) parseValue(value string, varType VarType) (interface{}, error) {
	switch varType {
	case VarTypeString:
		return value, nil

	case VarTypeNumber:
		// Try parsing as float first
		if f, err := strconv.ParseFloat(value, 64); err == nil {
			return f, nil
		}
		// Try parsing as int
		if i, err := strconv.ParseInt(value, 10, 64); err == nil {
			return i, nil
		}
		return nil, fmt.Errorf("invalid number: %s", value)

	case VarTypeBoolean:
		b, err := strconv.ParseBool(value)
		if err != nil {
			return nil, fmt.Errorf("invalid boolean: %s", value)
		}
		return b, nil

	case VarTypeObject:
		var obj interface{}
		if err := json.Unmarshal([]byte(value), &obj); err != nil {
			return nil, fmt.Errorf("invalid JSON object: %w", err)
		}
		return obj, nil

	default:
		return value, nil
	}
}

// validateType validates that a value matches the expected type
func (r *Renderer) validateType(value interface{}, varType VarType) error {
	switch varType {
	case VarTypeString:
		if _, ok := value.(string); !ok {
			return fmt.Errorf("expected string, got %T", value)
		}

	case VarTypeNumber:
		switch value.(type) {
		case int, int8, int16, int32, int64,
			uint, uint8, uint16, uint32, uint64,
			float32, float64:
			// OK
		default:
			return fmt.Errorf("expected number, got %T", value)
		}

	case VarTypeBoolean:
		if _, ok := value.(bool); !ok {
			return fmt.Errorf("expected boolean, got %T", value)
		}

	case VarTypeObject:
		switch value.(type) {
		case map[string]interface{}, []interface{}:
			// OK
		default:
			return fmt.Errorf("expected object/array, got %T", value)
		}
	}

	return nil
}

// valueToString converts a value to its string representation
func (r *Renderer) valueToString(value interface{}) (string, error) {
	switch v := value.(type) {
	case string:
		return v, nil

	case int, int8, int16, int32, int64:
		return fmt.Sprintf("%d", v), nil

	case uint, uint8, uint16, uint32, uint64:
		return fmt.Sprintf("%d", v), nil

	case float32, float64:
		return fmt.Sprintf("%v", v), nil

	case bool:
		return strconv.FormatBool(v), nil

	case map[string]interface{}, []interface{}:
		// Convert objects/arrays to JSON
		b, err := json.Marshal(v)
		if err != nil {
			return "", err
		}
		return string(b), nil

	default:
		return fmt.Sprintf("%v", v), nil
	}
}
