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
	Requirements contracts.Requirements
	Providers    []Provider
	HTTPClient   *http.Client
	Limit        int
}

type HTTPProviderConfig struct {
	Client  *http.Client
	BaseURL string
}

func QueryFromRequirements(req contracts.Requirements, limit int) Query {
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
