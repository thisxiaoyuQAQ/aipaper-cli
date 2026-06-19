package search

import (
	"context"
	"net/url"
	"strconv"

	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/contracts"
)

const semanticScholarBaseURL = "https://api.semanticscholar.org/graph/v1"

type SemanticScholarProvider struct {
	httpProvider
}

func NewSemanticScholarProvider(cfg HTTPProviderConfig) SemanticScholarProvider {
	baseURL := cfg.BaseURL
	if baseURL == "" {
		baseURL = semanticScholarBaseURL
	}
	return SemanticScholarProvider{httpProvider: httpProvider{
		name:    SemanticScholarProviderName,
		baseURL: baseURL,
		client:  defaultHTTPClient(cfg.Client),
	}}
}

func (p SemanticScholarProvider) Search(ctx context.Context, query Query) ([]contracts.ReferenceCandidate, error) {
	var resp struct {
		Data []struct {
			Title         string `json:"title"`
			Year          int    `json:"year"`
			Abstract      string `json:"abstract"`
			Venue         string `json:"venue"`
			CitationCount int    `json:"citationCount"`
			URL           string `json:"url"`
			ExternalIDs   struct {
				DOI string `json:"DOI"`
			} `json:"externalIds"`
			Authors []struct {
				Name string `json:"name"`
			} `json:"authors"`
		} `json:"data"`
	}
	values := url.Values{}
	values.Set("query", query.Text)
	values.Set("limit", strconv.Itoa(query.Limit))
	values.Set("fields", "title,authors,year,externalIds,abstract,venue,citationCount,url")
	if err := p.getJSON(ctx, "/paper/search", values, &resp); err != nil {
		return nil, err
	}
	candidates := make([]contracts.ReferenceCandidate, 0, len(resp.Data))
	for _, item := range resp.Data {
		candidate := contracts.ReferenceCandidate{
			Title:         item.Title,
			Authors:       namesFromSemanticAuthors(item.Authors),
			Year:          item.Year,
			Source:        p.Name(),
			DOI:           item.ExternalIDs.DOI,
			URL:           item.URL,
			Abstract:      item.Abstract,
			Venue:         item.Venue,
			CitationCount: item.CitationCount,
			Reliability:   "official_api",
			Availability:  candidateAvailability(item.URL, item.ExternalIDs.DOI),
			AccessURL:     item.URL,
		}
		if err := validateCandidate(candidate); err != nil {
			return nil, ProviderError{Code: CodeSearchFieldMissing, Source: p.Name(), Message: err.Error(), Retryable: false}
		}
		candidates = append(candidates, withStatusAndDedupe(candidate))
	}
	return candidates, nil
}
