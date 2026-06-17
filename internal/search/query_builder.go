package search

import (
	"strings"

	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/contracts"
)

// buildComparisonQueries builds queries for Chinese historical comparison topics
func buildComparisonQueries(req contracts.Requirements, strategy QueryStrategy, limit int) []QueryWithMetadata {
	topic := strings.TrimSpace(req.Topic)
	var queries []QueryWithMetadata

	// Base query: remove comparison words
	cleanTopic := removeComparisonWords(topic)
	queries = append(queries, QueryWithMetadata{
		Query: Query{
			Text:     cleanTopic + " 研究",
			Scope:    req.Scope,
			Language: strategy.Language,
			Limit:    limit,
		},
		Strategy: strategy,
		Reason:   "Chinese historical topic - core research query",
	})

	// Query 2: Dynasty + entity type + evaluation
	dynasties := ExtractEntitiesByType(strategy.Entities, EntityTypeDynasty)
	if len(dynasties) > 0 {
		dynasty := dynasties[0].Text
		queries = append(queries, QueryWithMetadata{
			Query: Query{
				Text:     dynasty + "皇帝 评价",
				Scope:    req.Scope,
				Language: strategy.Language,
				Limit:    limit,
			},
			Strategy: strategy,
			Reason:   "Dynasty-specific evaluation query",
		})
	}

	// Query 3: Dynasty + politics + historical research
	if len(dynasties) > 0 {
		dynasty := strings.TrimSuffix(dynasties[0].Text, "朝")
		queries = append(queries, QueryWithMetadata{
			Query: Query{
				Text:     dynasty + " 政治 史学研究",
				Scope:    req.Scope,
				Language: strategy.Language,
				Limit:    limit,
			},
			Strategy: strategy,
			Reason:   "Historical political research query",
		})
	}

	// Query 4: Person names comparison (if mentioned in research questions)
	if len(req.ResearchQuestions) > 0 {
		for _, question := range req.ResearchQuestions {
			persons := RecognizeEntities(question, strategy.Language)
			personList := ExtractEntitiesByType(persons, EntityTypePerson)
			if len(personList) >= 2 {
				queries = append(queries, QueryWithMetadata{
					Query: Query{
						Text:     personList[0].Text + " " + personList[1].Text + " 比较",
						Scope:    req.Scope,
						Language: strategy.Language,
						Limit:    limit,
					},
					Strategy: strategy,
					Reason:   "Person-to-person comparison query",
				})
				break
			}
		}
	}

	return queries
}

// buildGeneralQueries builds queries for Chinese general academic topics
func buildGeneralQueries(req contracts.Requirements, strategy QueryStrategy, limit int) []QueryWithMetadata {
	topic := strings.TrimSpace(req.Topic)
	var queries []QueryWithMetadata

	if strategy.Language == "zh-CN" {
		// Chinese general academic queries
		queries = append(queries, QueryWithMetadata{
			Query: Query{
				Text:     topic + " 综述",
				Scope:    req.Scope,
				Language: strategy.Language,
				Limit:    limit,
			},
			Strategy: strategy,
			Reason:   "Chinese literature review query",
		})

		queries = append(queries, QueryWithMetadata{
			Query: Query{
				Text:     topic + " 研究现状",
				Scope:    req.Scope,
				Language: strategy.Language,
				Limit:    limit,
			},
			Strategy: strategy,
			Reason:   "Chinese research status query",
		})

		// Include research questions
		if len(req.ResearchQuestions) > 0 {
			for _, question := range req.ResearchQuestions {
				question = strings.TrimSpace(question)
				if question != "" {
					queries = append(queries, QueryWithMetadata{
						Query: Query{
							Text:     question + " 文献综述",
							Scope:    req.Scope,
							Language: strategy.Language,
							Limit:    limit,
						},
						Strategy: strategy,
						Reason:   "Research question literature review",
					})
					break // Only add one research question query for base
				}
			}
		}
	} else {
		// English general academic queries (keep existing logic)
		queries = buildDefaultQueries(req, strategy, limit)
	}

	return queries
}

// buildDefaultQueries builds queries for English academic topics
func buildDefaultQueries(req contracts.Requirements, strategy QueryStrategy, limit int) []QueryWithMetadata {
	topic := strings.TrimSpace(req.Topic)
	var queries []QueryWithMetadata

	// Base query: just the topic
	queries = append(queries, QueryWithMetadata{
		Query: Query{
			Text:     topic,
			Scope:    req.Scope,
			Language: strategy.Language,
			Limit:    limit,
		},
		Strategy: strategy,
		Reason:   "Base topic query",
	})

	// Add systematic review query
	queries = append(queries, QueryWithMetadata{
		Query: Query{
			Text:     topic + " systematic review",
			Scope:    req.Scope,
			Language: strategy.Language,
			Limit:    limit,
		},
		Strategy: strategy,
		Reason:   "Systematic review query",
	})

	// Add research questions with evidence suffix
	if len(req.ResearchQuestions) > 0 {
		for _, question := range req.ResearchQuestions {
			question = strings.TrimSpace(question)
			if question != "" {
				queries = append(queries, QueryWithMetadata{
					Query: Query{
						Text:     question + " evidence literature review",
						Scope:    req.Scope,
						Language: strategy.Language,
						Limit:    limit,
					},
					Strategy: strategy,
					Reason:   "Research question evidence query",
				})
				break
			}
		}
	}

	return queries
}

// buildComparisonExpansionQueries builds expansion queries for comparison topics
func buildComparisonExpansionQueries(req contracts.Requirements, strategy QueryStrategy, limit int, needed int) []QueryWithMetadata {
	// For comparison topics, use similar logic as base but with variations
	return buildComparisonQueries(req, strategy, limit)
}

// buildGeneralExpansionQueries builds expansion queries for general topics
func buildGeneralExpansionQueries(req contracts.Requirements, strategy QueryStrategy, limit int, needed int) []QueryWithMetadata {
	topic := strings.TrimSpace(req.Topic)
	var queries []QueryWithMetadata

	if strategy.Language == "zh-CN" {
		// Add empirical research query
		queries = append(queries, QueryWithMetadata{
			Query: Query{
				Text:     topic + " 实证研究",
				Scope:    req.Scope,
				Language: strategy.Language,
				Limit:    limit,
			},
			Strategy: strategy,
			Reason:   "Empirical research query",
		})

		// Add remaining research questions
		for i, question := range req.ResearchQuestions {
			if i > 0 { // Skip first one (already in base)
				question = strings.TrimSpace(question)
				if question != "" {
					queries = append(queries, QueryWithMetadata{
						Query: Query{
							Text:     question + " 文献",
							Scope:    req.Scope,
							Language: strategy.Language,
							Limit:    limit,
						},
						Strategy: strategy,
						Reason:   "Research question literature query",
					})
				}
			}
		}

		// Add chapter preferences
		for _, pref := range req.ChapterPreferences {
			pref = strings.TrimSpace(pref)
			if pref != "" {
				queries = append(queries, QueryWithMetadata{
					Query: Query{
						Text:     topic + " " + pref,
						Scope:    req.Scope,
						Language: strategy.Language,
						Limit:    limit,
					},
					Strategy: strategy,
					Reason:   "Chapter preference query",
				})
			}
		}
	} else {
		// English expansion
		queries = buildDefaultExpansionQueries(req, strategy, limit, needed)
	}

	return queries
}

// buildDefaultExpansionQueries builds expansion queries for English topics
func buildDefaultExpansionQueries(req contracts.Requirements, strategy QueryStrategy, limit int, needed int) []QueryWithMetadata {
	topic := strings.TrimSpace(req.Topic)
	var queries []QueryWithMetadata

	// Add empirical study
	queries = append(queries, QueryWithMetadata{
		Query: Query{
			Text:     topic + " empirical study",
			Scope:    req.Scope,
			Language: strategy.Language,
			Limit:    limit,
		},
		Strategy: strategy,
		Reason:   "Empirical study query",
	})

	// Add chapter preferences
	for _, pref := range req.ChapterPreferences {
		pref = strings.TrimSpace(pref)
		if pref != "" {
			queries = append(queries, QueryWithMetadata{
				Query: Query{
					Text:     topic + " " + pref,
					Scope:    req.Scope,
					Language: strategy.Language,
					Limit:    limit,
				},
				Strategy: strategy,
				Reason:   "Chapter preference query",
			})
		}
	}

	// Add remaining research questions
	for i, question := range req.ResearchQuestions {
		if i > 0 {
			question = strings.TrimSpace(question)
			if question != "" {
				queries = append(queries, QueryWithMetadata{
					Query: Query{
						Text:     question + " literature",
						Scope:    req.Scope,
						Language: strategy.Language,
						Limit:    limit,
					},
					Strategy: strategy,
					Reason:   "Research question literature",
				})
			}
		}
	}

	return queries
}

// removeComparisonWords removes comparison markers from text
func removeComparisonWords(text string) string {
	for _, marker := range comparisonKeywords {
		text = strings.ReplaceAll(text, marker, "")
	}
	// Clean up multiple spaces
	text = strings.Join(strings.Fields(text), " ")
	return strings.TrimSpace(text)
}
