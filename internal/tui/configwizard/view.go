package configwizard

import (
	"fmt"
	"strings"

	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/config"
	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/i18n"
)

func (m Model) View() string {
	var b strings.Builder
	b.WriteString(m.i18n.Text(i18n.ConfigTitle))
	b.WriteString("\n\n")
	switch m.step {
	case StepTemplate:
		b.WriteString(m.i18n.Text(i18n.ConfigProviderTemplate))
		b.WriteString("\n\n")
		for i, template := range DefaultTemplates() {
			cursor := " "
			if i == m.templateIdx {
				cursor = ">"
			}
			fmt.Fprintf(&b, "%s %s\n", cursor, template.Name)
		}
	case StepFields:
		b.WriteString(m.i18n.Text(i18n.ConfigProviderSettings))
		b.WriteString("\n\n")
		for i, field := range orderedFields {
			cursor := " "
			if i == m.cursor {
				cursor = ">"
			}
			fmt.Fprintf(&b, "%s %s: %s\n", cursor, m.fieldLabel(field), m.displayField(field))
		}
		if m.directSecret {
			b.WriteString("\n")
			b.WriteString(m.i18n.Text(i18n.ConfigDirectSecretWarning))
			b.WriteString("\n")
		}
	case StepSummary:
		b.WriteString(m.i18n.Text(i18n.ConfigSummaryTitle))
		b.WriteString("\n\n")
		b.WriteString(m.Summary())
		b.WriteString("\n")
		if m.directSecret {
			b.WriteString("\n")
			b.WriteString(m.i18n.Text(i18n.ConfigDirectSecretWarning))
			b.WriteString("\n")
		}
	}
	if m.err != nil {
		fmt.Fprintf(&b, "\n%s: %s\n", m.i18n.Text(i18n.CommonErrorPrefix), m.err)
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

func (m Model) fieldLabel(field Field) string {
	switch field {
	case FieldProviderName:
		return m.i18n.Text(i18n.ConfigFieldProviderName)
	case FieldProviderType:
		return m.i18n.Text(i18n.ConfigFieldProviderType)
	case FieldBaseURL:
		return m.i18n.Text(i18n.ConfigFieldBaseURL)
	case FieldModel:
		return m.i18n.Text(i18n.ConfigFieldModel)
	case FieldAPIKey:
		return m.i18n.Text(i18n.ConfigFieldAPIKey)
	case FieldDefaultLanguage:
		return m.i18n.Text(i18n.ConfigFieldDefaultLanguage)
	case FieldUILanguage:
		return m.i18n.Text(i18n.ConfigFieldUILanguage)
	case FieldCitationStyle:
		return m.i18n.Text(i18n.ConfigFieldCitationStyle)
	default:
		return m.i18n.Text(i18n.CommonUnknown)
	}
}
