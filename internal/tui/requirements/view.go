package requirements

import (
	"fmt"
	"strings"

	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/i18n"
)

func (m Model) View() string {
	var b strings.Builder
	b.WriteString(m.i18n.Text(i18n.RequirementsTitle))
	b.WriteString("\n\n")
	for i, field := range orderedFields {
		cursor := " "
		if i == m.cursor {
			cursor = ">"
		}
		value := strings.ReplaceAll(m.values[field], "\n", " / ")
		fmt.Fprintf(&b, "%s %s: %s\n", cursor, m.fieldLabel(field), value)
	}
	if m.err != nil {
		fmt.Fprintf(&b, "\n%s: %s\n", m.i18n.Text(i18n.CommonErrorPrefix), m.err)
	}
	return b.String()
}

func (m Model) fieldLabel(field Field) string {
	switch field {
	case FieldTopic:
		return m.i18n.Text(i18n.RequirementsFieldTopic)
	case FieldResearchQuestions:
		return m.i18n.Text(i18n.RequirementsFieldResearchQuestions)
	case FieldScope:
		return m.i18n.Text(i18n.RequirementsFieldScope)
	case FieldLanguage:
		return m.i18n.Text(i18n.RequirementsFieldLanguage)
	case FieldCitationStyle:
		return m.i18n.Text(i18n.RequirementsFieldCitationStyle)
	case FieldArticleTemplate:
		return m.i18n.Text(i18n.RequirementsFieldArticleTemplate)
	case FieldQualityMode:
		return m.i18n.Text(i18n.RequirementsFieldQualityMode)
	case FieldTargetWords:
		return m.i18n.Text(i18n.RequirementsFieldTargetWords)
	case FieldMaterialDir:
		return m.i18n.Text(i18n.RequirementsFieldMaterialDir)
	case FieldAllowOnlineSearch:
		return m.i18n.Text(i18n.RequirementsFieldAllowOnlineSearch)
	case FieldSearchProviders:
		return m.i18n.Text(i18n.RequirementsFieldSearchProviders)
	case FieldChapterPreferences:
		return m.i18n.Text(i18n.RequirementsFieldChapterPreferences)
	case FieldConstraints:
		return m.i18n.Text(i18n.RequirementsFieldConstraints)
	default:
		return m.i18n.Text(i18n.CommonUnknown)
	}
}
