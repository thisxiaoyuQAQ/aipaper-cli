package configwizard

import (
	"fmt"
	"strings"

	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/config"
)

func (m Model) View() string {
	var b strings.Builder
	b.WriteString("aipaper-cli\n\n")
	switch m.step {
	case StepTemplate:
		b.WriteString("Provider template\n\n")
		for i, template := range DefaultTemplates() {
			cursor := " "
			if i == m.templateIdx {
				cursor = ">"
			}
			fmt.Fprintf(&b, "%s %s\n", cursor, template.Name)
		}
	case StepFields:
		b.WriteString("Provider settings\n\n")
		for i, field := range orderedFields {
			cursor := " "
			if i == m.cursor {
				cursor = ">"
			}
			fmt.Fprintf(&b, "%s %s: %s\n", cursor, fieldLabel(field), m.displayField(field))
		}
		if m.directSecret {
			b.WriteString("\nAPI key will be saved directly. Prefer an env: reference.\n")
		}
	case StepSummary:
		b.WriteString("Configuration summary\n\n")
		b.WriteString(m.Summary())
		b.WriteString("\n")
		if m.directSecret {
			b.WriteString("\nAPI key will be saved directly. Prefer an env: reference.\n")
		}
	}
	if m.err != nil {
		fmt.Fprintf(&b, "\nError: %s\n", m.err)
	}
	return b.String()
}

func (m Model) displayField(field Field) string {
	value := m.values[field]
	if field == FieldAPIKey {
		return config.MaskSecret(value)
	}
	return value
}

func fieldLabel(field Field) string {
	switch field {
	case FieldProviderName:
		return "Provider name"
	case FieldProviderType:
		return "Provider type"
	case FieldBaseURL:
		return "Base URL"
	case FieldModel:
		return "Model"
	case FieldAPIKey:
		return "API key"
	case FieldDefaultLanguage:
		return "Default language"
	case FieldCitationStyle:
		return "Citation style"
	default:
		return "Unknown"
	}
}
