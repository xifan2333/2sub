package prompt

import (
	"encoding/xml"
	"fmt"
	"regexp"
	"strings"
)

var (
	// variablePattern matches {{ variable }} syntax
	variablePattern = regexp.MustCompile(`\{\{\s*([a-zA-Z_][a-zA-Z0-9_\.]*)\s*\}\}`)

	// metaPattern matches the meta element
	metaPattern = regexp.MustCompile(`(?s)<meta[^>]*>.*?</meta>`)
)

// Parser handles POML template parsing
type Parser struct{}

// NewParser creates a new POML parser
func NewParser() *Parser {
	return &Parser{}
}

// Parse parses a POML template string
func (p *Parser) Parse(poml string) (*Template, error) {
	template := &Template{
		Raw:       poml,
		Variables: make(map[string]*Variable),
		Content:   poml,
	}

	// Extract and parse meta element
	if err := p.extractMetadata(template); err != nil {
		return nil, fmt.Errorf("failed to extract metadata: %w", err)
	}

	// Extract variables from template content
	p.extractVariablesFromContent(template)

	return template, nil
}

// extractMetadata extracts and parses the meta element
func (p *Parser) extractMetadata(template *Template) error {
	metaMatch := metaPattern.FindString(template.Raw)
	if metaMatch == "" {
		// No meta element found, that's ok
		return nil
	}

	// Wrap in a root element for XML parsing
	wrapped := "<root>" + metaMatch + "</root>"

	var root struct {
		Meta Meta `xml:"meta"`
	}

	if err := xml.Unmarshal([]byte(wrapped), &root); err != nil {
		// If parsing fails, try to extract variables element manually
		return p.extractVariablesManually(template, metaMatch)
	}

	// Process variable definitions
	for i := range root.Meta.Variables.Vars {
		v := &root.Meta.Variables.Vars[i]

		// Set defaults
		if v.Type == "" {
			v.Type = VarTypeString
		}

		template.Variables[v.Name] = v
	}

	return nil
}

// extractVariablesManually attempts to extract variable definitions manually
// when XML parsing fails (e.g., due to mixed content)
func (p *Parser) extractVariablesManually(template *Template, metaContent string) error {
	// Look for <variables> section
	varPattern := regexp.MustCompile(`<variables>(.*?)</variables>`)
	varMatch := varPattern.FindStringSubmatch(metaContent)
	if len(varMatch) < 2 {
		return nil // No variables section
	}

	// Extract individual <var> elements
	varElemPattern := regexp.MustCompile(`<var\s+([^/>]+)/?\s*>`)
	matches := varElemPattern.FindAllStringSubmatch(varMatch[1], -1)

	for _, match := range matches {
		if len(match) < 2 {
			continue
		}

		v, err := p.parseVarAttributes(match[1])
		if err != nil {
			return err
		}

		template.Variables[v.Name] = v
	}

	return nil
}

// parseVarAttributes parses attributes from a var element
func (p *Parser) parseVarAttributes(attrs string) (*Variable, error) {
	v := &Variable{
		Type: VarTypeString, // default type
	}

	// Parse name
	namePattern := regexp.MustCompile(`name\s*=\s*"([^"]+)"`)
	if match := namePattern.FindStringSubmatch(attrs); len(match) > 1 {
		v.Name = match[1]
	} else {
		return nil, fmt.Errorf("variable missing name attribute")
	}

	// Parse required
	if strings.Contains(attrs, `required="true"`) {
		v.Required = true
	}

	// Parse default
	defaultPattern := regexp.MustCompile(`default\s*=\s*"([^"]*)"`)
	if match := defaultPattern.FindStringSubmatch(attrs); len(match) > 1 {
		v.Default = match[1]
	}

	// Parse type
	typePattern := regexp.MustCompile(`type\s*=\s*"([^"]+)"`)
	if match := typePattern.FindStringSubmatch(attrs); len(match) > 1 {
		v.Type = VarType(match[1])
	}

	// Parse description
	descPattern := regexp.MustCompile(`description\s*=\s*"([^"]*)"`)
	if match := descPattern.FindStringSubmatch(attrs); len(match) > 1 {
		v.Description = match[1]
	}

	return v, nil
}

// extractVariablesFromContent finds all {{ variable }} references in the template
func (p *Parser) extractVariablesFromContent(template *Template) {
	matches := variablePattern.FindAllStringSubmatch(template.Content, -1)

	for _, match := range matches {
		if len(match) < 2 {
			continue
		}

		varName := match[1]

		// Skip complex expressions (e.g., calculations, function calls)
		if strings.ContainsAny(varName, "+- */()[]") {
			continue
		}

		// If variable not already defined in metadata, create a basic entry
		if _, exists := template.Variables[varName]; !exists {
			template.Variables[varName] = &Variable{
				Name:     varName,
				Required: false,
				Type:     VarTypeString,
			}
		}
	}
}

// GetVariableNames returns all variable names found in the template
func (p *Parser) GetVariableNames(template *Template) []string {
	names := make([]string, 0, len(template.Variables))
	for name := range template.Variables {
		names = append(names, name)
	}
	return names
}

// GetRequiredVariables returns all required variables
func (p *Parser) GetRequiredVariables(template *Template) []*Variable {
	var required []*Variable
	for _, v := range template.Variables {
		if v.Required {
			required = append(required, v)
		}
	}
	return required
}
