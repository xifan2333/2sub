package prompt

import "encoding/xml"

// VarType represents the type of a variable
type VarType string

const (
	VarTypeString  VarType = "string"
	VarTypeNumber  VarType = "number"
	VarTypeBoolean VarType = "boolean"
	VarTypeObject  VarType = "object"
)

// Variable defines metadata for a template variable
type Variable struct {
	Name        string  `xml:"name,attr"`
	Required    bool    `xml:"required,attr"`
	Default     string  `xml:"default,attr"`
	Type        VarType `xml:"type,attr"`
	Description string  `xml:"description,attr"`
}

// Variables is a container for variable definitions in POML meta
type Variables struct {
	XMLName xml.Name   `xml:"variables"`
	Vars    []Variable `xml:"var"`
}

// Meta represents the POML meta element with extended variable definitions
type Meta struct {
	XMLName       xml.Name  `xml:"meta"`
	MinVersion    string    `xml:"minVersion,attr,omitempty"`
	MaxVersion    string    `xml:"maxVersion,attr,omitempty"`
	Components    string    `xml:"components,attr,omitempty"`
	Variables     Variables `xml:"variables"`
	InnerXML      []byte    `xml:",innerxml"`
}

// Template represents a parsed POML template
type Template struct {
	Raw       string            // Original POML content
	Variables map[string]*Variable // Variable metadata indexed by name
	Content   string            // Template content for rendering
}

// RenderContext contains values for rendering a template
type RenderContext struct {
	Values map[string]interface{}
}

// ValidationError represents a template validation error
type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	return e.Field + ": " + e.Message
}

// ValidationErrors is a collection of validation errors
type ValidationErrors []ValidationError

func (e ValidationErrors) Error() string {
	if len(e) == 0 {
		return ""
	}
	msg := "validation errors:"
	for _, err := range e {
		msg += "\n  - " + err.Error()
	}
	return msg
}
