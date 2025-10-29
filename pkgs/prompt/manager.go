package prompt

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// Manager manages POML prompt templates
type Manager struct {
	parser   *Parser
	renderer *Renderer
	cache    map[string]*Template
	mu       sync.RWMutex
}

// NewManager creates a new prompt manager
func NewManager() *Manager {
	return &Manager{
		parser:   NewParser(),
		renderer: NewRenderer(),
		cache:    make(map[string]*Template),
	}
}

// LoadTemplate loads a POML template from a string
func (m *Manager) LoadTemplate(poml string) (*Template, error) {
	return m.parser.Parse(poml)
}

// LoadTemplateFile loads a POML template from a file
func (m *Manager) LoadTemplateFile(path string) (*Template, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read template file: %w", err)
	}

	template, err := m.parser.Parse(string(content))
	if err != nil {
		return nil, fmt.Errorf("failed to parse template: %w", err)
	}

	return template, nil
}

// LoadTemplateFileWithCache loads a template from file with caching
func (m *Manager) LoadTemplateFileWithCache(path string) (*Template, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve path: %w", err)
	}

	// Check cache first
	m.mu.RLock()
	if template, exists := m.cache[absPath]; exists {
		m.mu.RUnlock()
		return template, nil
	}
	m.mu.RUnlock()

	// Load template
	template, err := m.LoadTemplateFile(absPath)
	if err != nil {
		return nil, err
	}

	// Cache it
	m.mu.Lock()
	m.cache[absPath] = template
	m.mu.Unlock()

	return template, nil
}

// ClearCache clears the template cache
func (m *Manager) ClearCache() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.cache = make(map[string]*Template)
}

// Render renders a template with the given values
func (m *Manager) Render(template *Template, values map[string]interface{}) (string, error) {
	ctx := &RenderContext{
		Values: values,
	}

	return m.renderer.Render(template, ctx)
}

// RenderString parses and renders a POML template string in one call
func (m *Manager) RenderString(poml string, values map[string]interface{}) (string, error) {
	template, err := m.LoadTemplate(poml)
	if err != nil {
		return "", err
	}

	return m.Render(template, values)
}

// RenderFile parses and renders a POML template file in one call
func (m *Manager) RenderFile(path string, values map[string]interface{}) (string, error) {
	template, err := m.LoadTemplateFile(path)
	if err != nil {
		return "", err
	}

	return m.Render(template, values)
}

// GetVariables returns all variables defined in a template
func (m *Manager) GetVariables(template *Template) []*Variable {
	vars := make([]*Variable, 0, len(template.Variables))
	for _, v := range template.Variables {
		vars = append(vars, v)
	}
	return vars
}

// GetRequiredVariables returns all required variables in a template
func (m *Manager) GetRequiredVariables(template *Template) []*Variable {
	return m.parser.GetRequiredVariables(template)
}

// ValidateContext validates a render context against a template
func (m *Manager) ValidateContext(template *Template, values map[string]interface{}) error {
	ctx := &RenderContext{
		Values: values,
	}

	return m.renderer.validateAndApplyDefaults(template, ctx)
}

// GetDefaultValues returns a map of default values for all variables
func (m *Manager) GetDefaultValues(template *Template) map[string]interface{} {
	defaults := make(map[string]interface{})

	for name, v := range template.Variables {
		if v.Default != "" {
			value, err := m.renderer.parseValue(v.Default, v.Type)
			if err == nil {
				defaults[name] = value
			}
		}
	}

	return defaults
}
