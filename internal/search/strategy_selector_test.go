package search

import (
	"reflect"
	"testing"

	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/contracts"
)

func TestSelectStrategy(t *testing.T) {
	tests := []struct {
		name     string
		req      contracts.Requirements
		expected QueryStrategy
	}{
		{
			name: "Chinese comparison topic",
			req: contracts.Requirements{
				Topic:    "明朝君主哪个更厉害",
				Language: "zh-CN",
			},
			expected: QueryStrategy{
				Language:  "zh-CN",
				TopicType: TopicTypeComparison,
				Entities: []Entity{
					{Text: "明朝", Type: EntityTypeDynasty, Start: 0, End: 2},
					{Text: "哪个", Type: EntityTypeComparison, Start: 4, End: 6},
				},
				Keywords: nil,
			},
		},
		{
			name: "Chinese general topic",
			req: contracts.Requirements{
				Topic:    "深度学习在医学影像中的应用",
				Language: "zh-CN",
				Scope:    "computer science",
			},
			expected: QueryStrategy{
				Language:  "zh-CN",
				TopicType: TopicTypeGeneral,
				Entities:  nil,
				Keywords:  []string{"computer science"},
			},
		},
		{
			name: "English general topic",
			req: contracts.Requirements{
				Topic:    "Retrieval augmented generation",
				Language: "en",
			},
			expected: QueryStrategy{
				Language:  "en",
				TopicType: TopicTypeGeneral,
				Entities:  nil,
				Keywords:  nil,
			},
		},
		{
			name: "Auto-detect Chinese",
			req: contracts.Requirements{
				Topic: "明朝历史研究",
				// No language specified
			},
			expected: QueryStrategy{
				Language: "zh-CN",
				TopicType: TopicTypeGeneral,
				Entities: []Entity{
					{Text: "明朝", Type: EntityTypeDynasty, Start: 0, End: 2},
				},
				Keywords: nil,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SelectStrategy(tt.req)

			if result.Language != tt.expected.Language {
				t.Errorf("Language = %q, want %q", result.Language, tt.expected.Language)
			}
			if result.TopicType != tt.expected.TopicType {
				t.Errorf("TopicType = %q, want %q", result.TopicType, tt.expected.TopicType)
			}
			if !entitiesEqual(result.Entities, tt.expected.Entities) {
				t.Errorf("Entities = %+v, want %+v", result.Entities, tt.expected.Entities)
			}
			if !reflect.DeepEqual(result.Keywords, tt.expected.Keywords) {
				t.Errorf("Keywords = %+v, want %+v", result.Keywords, tt.expected.Keywords)
			}
		})
	}
}

func TestClassifyTopicType(t *testing.T) {
	tests := []struct {
		name     string
		topic    string
		entities []Entity
		expected TopicType
	}{
		{
			name:  "comparison with marker",
			topic: "明朝君主哪个更厉害",
			entities: []Entity{
				{Type: EntityTypeComparison},
			},
			expected: TopicTypeComparison,
		},
		{
			name:     "general topic",
			topic:    "深度学习应用",
			entities: nil,
			expected: TopicTypeGeneral,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := classifyTopicType(tt.topic, tt.entities)
			if result != tt.expected {
				t.Errorf("classifyTopicType() = %q, want %q", result, tt.expected)
			}
		})
	}
}
