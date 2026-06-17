package search

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/contracts"
)

const (
	CodeSearchTimeout       = "REFERENCE_SEARCH_TIMEOUT"
	CodeSearchRateLimited   = "REFERENCE_SEARCH_RATE_LIMITED"
	CodeSearchFieldMissing  = "REFERENCE_SEARCH_FIELD_MISSING"
	CodeCandidatesEmpty     = "REFERENCE_CANDIDATES_EMPTY"
	CodeProviderUnsupported = "CONFIG_PROVIDER_UNSUPPORTED"
)

type Query struct {
	Text     string
	Scope    string
	Language string
	Limit    int
}

type Provider interface {
	Name() string
	Search(ctx context.Context, query Query) ([]contracts.ReferenceCandidate, error)
}

type ProviderError struct {
	Code      string `json:"code"`
	Source    string `json:"source"`
	Message   string `json:"message"`
	Retryable bool   `json:"retryable"`
}

func (e ProviderError) Error() string {
	if e.Source == "" {
		return e.Message
	}
	return e.Source + ": " + e.Message
}

type Result struct {
	Candidates contracts.ReferenceCandidates `json:"candidates"`
	Errors     []ProviderError               `json:"errors,omitempty"`
	Outputs    []string                      `json:"outputs,omitempty"`
}

type Options struct {
	Requirements      contracts.Requirements
	Providers         []Provider
	HTTPClient        *http.Client
	Limit             int
	ExpansionEnabled  bool
	MinCandidateCount int
	ExpansionLimit    int
}

type HTTPProviderConfig struct {
	Client  *http.Client
	BaseURL string
}

// QueryFromRequirements generates a single query from requirements.
// Deprecated: Use GenerateQueries for better Chinese query support.
// Kept for backward compatibility.
func QueryFromRequirements(req contracts.Requirements, limit int) Query {
	queries := GenerateQueries(req, limit)
	if len(queries) > 0 {
		return queries[0].Query
	}
	// Fallback to old logic if no queries generated
	if limit <= 0 {
		limit = 10
	}
	parts := []string{strings.TrimSpace(req.Topic)}
	for _, question := range req.ResearchQuestions {
		if strings.TrimSpace(question) != "" {
			parts = append(parts, strings.TrimSpace(question))
		}
	}
	return Query{
		Text:     strings.Join(parts, " "),
		Scope:    req.Scope,
		Language: req.Language,
		Limit:    limit,
	}
}

// ExpansionQueriesFromRequirements generates expansion queries.
// Deprecated: Use GenerateExpansionQueries for better Chinese query support.
// Kept for backward compatibility.
func ExpansionQueriesFromRequirements(req contracts.Requirements, base Query, needed int) []Query {
	if needed <= 0 {
		return nil
	}

	// Convert base Query to QueryWithMetadata for new API
	baseWithMeta := QueryWithMetadata{
		Query:    base,
		Strategy: SelectStrategy(req),
	}

	queriesWithMeta := GenerateExpansionQueries(req, baseWithMeta, needed)

	// Convert back to plain Query slice
	var queries []Query
	for _, q := range queriesWithMeta {
		queries = append(queries, q.Query)
	}

	// Fallback to old logic if no queries generated
	if len(queries) == 0 {
		limit := base.Limit
		if limit <= 0 {
			limit = 10
		}
		var texts []string
		for _, preference := range req.ChapterPreferences {
			preference = strings.TrimSpace(preference)
			if preference != "" {
				texts = append(texts, strings.TrimSpace(req.Topic+" "+preference))
			}
		}
		for _, question := range req.ResearchQuestions {
			question = strings.TrimSpace(question)
			if question != "" {
				texts = append(texts, strings.TrimSpace(question+" evidence literature review"))
			}
		}
		if len(texts) == 0 && strings.TrimSpace(req.Topic) != "" {
			texts = append(texts, strings.TrimSpace(req.Topic+" systematic review empirical study"))
		}
		seen := map[string]bool{strings.ToLower(strings.TrimSpace(base.Text)): true}
		for _, text := range texts {
			key := strings.ToLower(strings.TrimSpace(text))
			if key == "" || seen[key] {
				continue
			}
			seen[key] = true
			queries = append(queries, Query{Text: text, Scope: req.Scope, Language: req.Language, Limit: limit})
			if len(queries) >= needed {
				break
			}
		}
	}

	return queries
}

func normalizeProviderError(source string, err error) ProviderError {
	var providerErr ProviderError
	if errors.As(err, &providerErr) {
		if providerErr.Source == "" {
			providerErr.Source = source
		}
		return providerErr
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return ProviderError{Code: CodeSearchTimeout, Source: source, Message: err.Error(), Retryable: true}
	}
	return ProviderError{Code: "REFERENCE_SEARCH_FAILED", Source: source, Message: err.Error(), Retryable: false}
}

func defaultHTTPClient(client *http.Client) *http.Client {
	if client != nil {
		return client
	}
	return &http.Client{Timeout: 20 * time.Second}
}

func validateCandidate(candidate contracts.ReferenceCandidate) error {
	if strings.TrimSpace(candidate.Title) == "" {
		return fmt.Errorf("%s: title is required", CodeSearchFieldMissing)
	}
	if strings.TrimSpace(candidate.Source) == "" {
		return fmt.Errorf("%s: source is required", CodeSearchFieldMissing)
	}
	return nil
}
