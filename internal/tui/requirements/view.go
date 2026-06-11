package requirements

import (
	"fmt"
	"strings"
)

func (m Model) View() string {
	var b strings.Builder
	b.WriteString("Writing requirements\n\n")
	for i, field := range orderedFields {
		cursor := " "
		if i == m.cursor {
			cursor = ">"
		}
		value := strings.ReplaceAll(m.values[field], "\n", " / ")
		fmt.Fprintf(&b, "%s %s: %s\n", cursor, fieldLabel(field), value)
	}
	if m.err != nil {
		fmt.Fprintf(&b, "\nError: %s\n", m.err)
	}
	return b.String()
}

func fieldLabel(field Field) string {
	switch field {
	case FieldTopic:
		return "Topic"
	case FieldResearchQuestions:
		return "Research questions"
	case FieldScope:
		return "Scope"
	case FieldLanguage:
		return "Language"
	case FieldCitationStyle:
		return "Citation style"
	case FieldQualityMode:
		return "Quality mode"
	case FieldTargetWords:
		return "Target words"
	case FieldMaterialDir:
		return "Material dir"
	case FieldAllowOnlineSearch:
		return "Allow online search"
	case FieldSearchProviders:
		return "Search providers"
	case FieldChapterPreferences:
		return "Chapter preferences"
	case FieldConstraints:
		return "Constraints"
	default:
		return "Unknown"
	}
}
