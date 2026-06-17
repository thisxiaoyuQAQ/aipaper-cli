package search

import (
	"reflect"
	"testing"
)

func TestRecognizeEntities(t *testing.T) {
	tests := []struct {
		name     string
		text     string
		language string
		expected []Entity
	}{
		{
			name:     "Chinese dynasty",
			text:     "明朝君主",
			language: "zh-CN",
			expected: []Entity{
				{Text: "明朝", Type: EntityTypeDynasty, Start: 0, End: 2},
			},
		},
		{
			name:     "comparison marker",
			text:     "哪个更厉害",
			language: "zh-CN",
			expected: []Entity{
				{Text: "哪个", Type: EntityTypeComparison, Start: 0, End: 2},
			},
		},
		{
			name:     "person names",
			text:     "朱元璋和朱棣",
			language: "zh-CN",
			expected: []Entity{
				{Text: "朱元璋", Type: EntityTypePerson, Start: 0, End: 3},
				{Text: "朱棣", Type: EntityTypePerson, Start: 4, End: 6},
			},
		},
		{
			name:     "combined: dynasty + comparison",
			text:     "明朝君主哪个更厉害",
			language: "zh-CN",
			expected: []Entity{
				{Text: "明朝", Type: EntityTypeDynasty, Start: 0, End: 2},
				{Text: "哪个", Type: EntityTypeComparison, Start: 4, End: 6},
			},
		},
		{
			name:     "no entities",
			text:     "深度学习应用",
			language: "zh-CN",
			expected: nil,
		},
		{
			name:     "English text - should return nil",
			text:     "Ming Dynasty comparison",
			language: "en",
			expected: nil,
		},
		{
			name:     "empty text",
			text:     "",
			language: "zh-CN",
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := RecognizeEntities(tt.text, tt.language)
			if !entitiesEqual(result, tt.expected) {
				t.Errorf("RecognizeEntities(%q, %q) = %+v, want %+v", tt.text, tt.language, result, tt.expected)
			}
		})
	}
}

func TestHasEntityType(t *testing.T) {
	entities := []Entity{
		{Text: "明朝", Type: EntityTypeDynasty, Start: 0, End: 2},
		{Text: "哪个", Type: EntityTypeComparison, Start: 4, End: 6},
	}

	tests := []struct {
		name       string
		entities   []Entity
		entityType EntityType
		expected   bool
	}{
		{
			name:       "has dynasty",
			entities:   entities,
			entityType: EntityTypeDynasty,
			expected:   true,
		},
		{
			name:       "has comparison",
			entities:   entities,
			entityType: EntityTypeComparison,
			expected:   true,
		},
		{
			name:       "no person",
			entities:   entities,
			entityType: EntityTypePerson,
			expected:   false,
		},
		{
			name:       "empty entities",
			entities:   nil,
			entityType: EntityTypeDynasty,
			expected:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := HasEntityType(tt.entities, tt.entityType)
			if result != tt.expected {
				t.Errorf("HasEntityType(..., %q) = %v, want %v", tt.entityType, result, tt.expected)
			}
		})
	}
}

func TestExtractEntitiesByType(t *testing.T) {
	entities := []Entity{
		{Text: "明朝", Type: EntityTypeDynasty, Start: 0, End: 2},
		{Text: "哪个", Type: EntityTypeComparison, Start: 4, End: 6},
		{Text: "清朝", Type: EntityTypeDynasty, Start: 8, End: 10},
	}

	tests := []struct {
		name       string
		entities   []Entity
		entityType EntityType
		expected   []Entity
	}{
		{
			name:       "extract dynasties",
			entities:   entities,
			entityType: EntityTypeDynasty,
			expected: []Entity{
				{Text: "明朝", Type: EntityTypeDynasty, Start: 0, End: 2},
				{Text: "清朝", Type: EntityTypeDynasty, Start: 8, End: 10},
			},
		},
		{
			name:       "extract comparison",
			entities:   entities,
			entityType: EntityTypeComparison,
			expected: []Entity{
				{Text: "哪个", Type: EntityTypeComparison, Start: 4, End: 6},
			},
		},
		{
			name:       "no match",
			entities:   entities,
			entityType: EntityTypePerson,
			expected:   nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ExtractEntitiesByType(tt.entities, tt.entityType)
			if !entitiesEqual(result, tt.expected) {
				t.Errorf("ExtractEntitiesByType(..., %q) = %+v, want %+v", tt.entityType, result, tt.expected)
			}
		})
	}
}

// Helper function to compare entity slices
func entitiesEqual(a, b []Entity) bool {
	if len(a) != len(b) {
		return false
	}
	if len(a) == 0 && len(b) == 0 {
		return true
	}
	return reflect.DeepEqual(a, b)
}
