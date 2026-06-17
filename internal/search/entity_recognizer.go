package search

import (
	"regexp"
	"strings"
)

// EntityType represents the type of recognized entity
type EntityType string

const (
	EntityTypeDynasty    EntityType = "Dynasty"
	EntityTypePerson     EntityType = "Person"
	EntityTypeComparison EntityType = "Comparison"
)

// Entity represents a recognized entity in the text
type Entity struct {
	Text  string     // The entity text
	Type  EntityType // Dynasty, Person, Comparison
	Start int        // Start position in original text (rune index)
	End   int        // End position in original text (rune index)
}

var (
	// Chinese dynasty names pattern
	dynastyPattern = regexp.MustCompile(`(夏|商|周|秦|汉|三国|晋|南北朝|隋|唐|五代|宋|元|明|清|民国)(朝)?`)

	// Comparison keywords
	comparisonKeywords = []string{"哪个", "谁", "比较", "对比", "vs", "versus", "VS"}

	// Common historical figures (extensible dictionary)
	personNames = map[string]bool{
		"朱元璋": true,
		"朱棣":  true,
		"康熙":  true,
		"乾隆":  true,
		"雍正":  true,
		"李世民": true,
		"武则天": true,
		"秦始皇": true,
		"汉武帝": true,
		"唐太宗": true,
	}
)

// RecognizeEntities identifies entities in the given text.
// Only processes Chinese text (language == "zh-CN").
func RecognizeEntities(text string, language string) []Entity {
	if language != "zh-CN" || text == "" {
		return nil
	}

	var entities []Entity

	// 1. Dynasty recognition
	entities = append(entities, recognizeDynasties(text)...)

	// 2. Comparison markers
	entities = append(entities, recognizeComparisons(text)...)

	// 3. Person names
	entities = append(entities, recognizePersons(text)...)

	return entities
}

func recognizeDynasties(text string) []Entity {
	var entities []Entity
	matches := dynastyPattern.FindAllStringIndex(text, -1)

	for _, match := range matches {
		start, end := match[0], match[1]
		matchText := text[start:end]

		// Skip if this is part of a person name (simple heuristic: check for common surname before dynasty character)
		if start > 0 {
			prevChar := []rune(text[:start])
			if len(prevChar) > 0 {
				lastChar := prevChar[len(prevChar)-1]
				// Common Chinese surnames that might be followed by dynasty characters
				if isCommonSurname(lastChar) {
					continue
				}
			}
		}

		// Convert byte positions to rune positions
		runeStart := len([]rune(text[:start]))
		runeEnd := len([]rune(text[:end]))

		entities = append(entities, Entity{
			Text:  matchText,
			Type:  EntityTypeDynasty,
			Start: runeStart,
			End:   runeEnd,
		})
	}

	return entities
}

// isCommonSurname checks if a character is a common Chinese surname
func isCommonSurname(r rune) bool {
	commonSurnames := []rune{'朱', '李', '王', '张', '刘', '陈', '杨', '黄', '赵', '周', '吴', '徐', '孙', '马', '胡', '郭', '林', '何', '高', '罗'}
	for _, surname := range commonSurnames {
		if r == surname {
			return true
		}
	}
	return false
}

func recognizeComparisons(text string) []Entity {
	var entities []Entity
	textLower := strings.ToLower(text)

	for _, keyword := range comparisonKeywords {
		keywordLower := strings.ToLower(keyword)
		index := strings.Index(textLower, keywordLower)

		if index != -1 {
			// Convert byte position to rune position
			runeStart := len([]rune(text[:index]))
			runeEnd := runeStart + len([]rune(keyword))

			entities = append(entities, Entity{
				Text:  keyword,
				Type:  EntityTypeComparison,
				Start: runeStart,
				End:   runeEnd,
			})
		}
	}

	return entities
}

func recognizePersons(text string) []Entity {
	var entities []Entity

	for name := range personNames {
		index := strings.Index(text, name)
		if index != -1 {
			// Convert byte position to rune position
			runeStart := len([]rune(text[:index]))
			runeEnd := runeStart + len([]rune(name))

			entities = append(entities, Entity{
				Text:  name,
				Type:  EntityTypePerson,
				Start: runeStart,
				End:   runeEnd,
			})
		}
	}

	return entities
}

// HasEntityType checks if the entity list contains any entity of the given type
func HasEntityType(entities []Entity, entityType EntityType) bool {
	for _, e := range entities {
		if e.Type == entityType {
			return true
		}
	}
	return false
}

// ExtractEntitiesByType returns all entities of the given type
func ExtractEntitiesByType(entities []Entity, entityType EntityType) []Entity {
	var result []Entity
	for _, e := range entities {
		if e.Type == entityType {
			result = append(result, e)
		}
	}
	return result
}
