package search

import (
	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/contracts"
)

// TopicType represents the classification of a topic
type TopicType string

const (
	TopicTypeComparison TopicType = "comparison"
	TopicTypeGeneral    TopicType = "general"
	TopicTypeReview     TopicType = "review"
)

// QueryStrategy contains the strategy for generating queries
type QueryStrategy struct {
	Language  string     // zh-CN, en
	TopicType TopicType  // comparison, general, review
	Entities  []Entity   // Recognized entities
	Keywords  []string   // Key terms extracted from topic
}

// SelectStrategy determines the appropriate query generation strategy
// based on the requirements and detected language/entities.
func SelectStrategy(req contracts.Requirements) QueryStrategy {
	// Detect language (prefer user-specified, fallback to detection)
	language := req.Language
	if language == "" {
		language = DetectLanguage(req.Topic)
	}

	// Recognize entities (only for Chinese)
	entities := RecognizeEntities(req.Topic, language)

	// Determine topic type
	topicType := classifyTopicType(req.Topic, entities)

	return QueryStrategy{
		Language:  language,
		TopicType: topicType,
		Entities:  entities,
		Keywords:  extractKeywords(req),
	}
}

// classifyTopicType determines the topic type based on entities and content
func classifyTopicType(topic string, entities []Entity) TopicType {
	// If has comparison marker, it's a comparison topic
	if HasEntityType(entities, EntityTypeComparison) {
		return TopicTypeComparison
	}

	// Default to general academic topic
	return TopicTypeGeneral
}

// extractKeywords extracts key terms from requirements
func extractKeywords(req contracts.Requirements) []string {
	var keywords []string

	// Add scope if available
	if req.Scope != "" {
		keywords = append(keywords, req.Scope)
	}

	// Could add more sophisticated keyword extraction here
	return keywords
}
