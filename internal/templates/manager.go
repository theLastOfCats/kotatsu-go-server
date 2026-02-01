package templates

import (
	"bytes"
	"fmt"
	"html/template"
	"path/filepath"
)

type Manager struct {
	templatesDir string
	cache        map[string]*template.Template
}

func NewManager(templatesDir string) *Manager {
	return &Manager{
		templatesDir: templatesDir,
		cache:        make(map[string]*template.Template),
	}
}

func (m *Manager) Render(templateName string, data interface{}) (string, error) {
	tmpl, ok := m.cache[templateName]
	if !ok {
		// Lazily load template
		path := filepath.Join(m.templatesDir, templateName)
		var err error
		tmpl, err = template.ParseFiles(path)
		if err != nil {
			return "", fmt.Errorf("failed to parse template %s: %w", templateName, err)
		}
		m.cache[templateName] = tmpl
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("failed to execute template %s: %w", templateName, err)
	}

	return buf.String(), nil
}
