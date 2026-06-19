package search

import (
	"context"
	"net/url"
	"strconv"
	"strings"

	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/contracts"
)

const crossrefBaseURL = "https://api.crossref.org"

type CrossrefProvider struct {
	httpProvider
}

func NewCrossrefProvider(cfg HTTPProviderConfig) CrossrefProvider {
	baseURL := cfg.BaseURL
	if baseURL == "" {
		baseURL = crossrefBaseURL
	}
	return CrossrefProvider{httpProvider: httpProvider{
		name:    CrossrefProviderName,
		baseURL: baseURL,
		client:  defaultHTTPClient(cfg.Client),
	}}
}

func (p CrossrefProvider) Search(ctx context.Context, query Query) ([]contracts.ReferenceCandidate, error) {
	var resp struct {
		Message struct {
			Items []struct {
				DOI                 string   `json:"DOI"`
				Title               []string `json:"title"`
				Abstract            string   `json:"abstract"`
				URL                 string   `json:"URL"`
				ContainerTitle      []string `json:"container-title"`
				IsReferencedByCount int      `json:"is-referenced-by-count"`
				Author              []struct {
					Given  string `json:"given"`
					Family string `json:"family"`
					Name   string `json:"name"`
				} `json:"author"`
				PublishedPrint struct {
					DateParts [][]int `json:"date-parts"`
				} `json:"published-print"`
				PublishedOnline struct {
					DateParts [][]int `json:"date-parts"`
				} `json:"published-online"`
				Issued struct {
					DateParts [][]int `json:"date-parts"`
				} `json:"issued"`
			} `json:"items"`
		} `json:"message"`
	}
	values := url.Values{}
	values.Set("query", query.Text)
	values.Set("rows", strconv.Itoa(query.Limit))
	if err := p.getJSON(ctx, "/works", values, &resp); err != nil {
		return nil, err
	}
	candidates := make([]contracts.ReferenceCandidate, 0, len(resp.Message.Items))
	for _, item := range resp.Message.Items {
		year := firstNonZero(
			yearFromDateParts(item.PublishedPrint.DateParts),
			yearFromDateParts(item.PublishedOnline.DateParts),
			yearFromDateParts(item.Issued.DateParts),
		)
		candidate := contracts.ReferenceCandidate{
			Title:         firstString(item.Title),
			Authors:       namesFromCrossrefAuthors(item.Author),
			Year:          year,
			Source:        p.Name(),
			DOI:           item.DOI,
			URL:           item.URL,
			Abstract:      cleanAbstract(item.Abstract),
			Venue:         firstString(item.ContainerTitle),
			CitationCount: item.IsReferencedByCount,
			Reliability:   "crossref_metadata",
			Availability:  candidateAvailability(item.URL, item.DOI),
			AccessURL:     item.URL,
		}
		if err := validateCandidate(candidate); err != nil {
			return nil, ProviderError{Code: CodeSearchFieldMissing, Source: p.Name(), Message: err.Error(), Retryable: false}
		}
		candidates = append(candidates, withStatusAndDedupe(candidate))
	}
	return candidates, nil
}

func namesFromCrossrefAuthors(authors []struct {
	Given  string `json:"given"`
	Family string `json:"family"`
	Name   string `json:"name"`
}) []string {
	out := make([]string, 0, len(authors))
	for _, author := range authors {
		name := strings.TrimSpace(author.Name)
		if name == "" {
			name = strings.TrimSpace(strings.TrimSpace(author.Given) + " " + strings.TrimSpace(author.Family))
		}
		if name != "" {
			out = append(out, name)
		}
	}
	return out
}

func firstNonZero(values ...int) int {
	for _, value := range values {
		if value != 0 {
			return value
		}
	}
	return 0
}
