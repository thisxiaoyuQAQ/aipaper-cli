package search

import (
	"context"
	"net/url"
	"strconv"
	"strings"

	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/contracts"
	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/references"
)

const doajBaseURL = "https://doaj.org"

type DOAJProvider struct {
	httpProvider
}

func NewDOAJProvider(cfg HTTPProviderConfig) DOAJProvider {
	baseURL := cfg.BaseURL
	if baseURL == "" {
		baseURL = doajBaseURL
	}
	return DOAJProvider{httpProvider: httpProvider{
		name:    DOAJProviderName,
		baseURL: baseURL,
		client:  defaultHTTPClient(cfg.Client),
	}}
}

func (p DOAJProvider) Search(ctx context.Context, query Query) ([]contracts.ReferenceCandidate, error) {
	var resp struct {
		Results []struct {
			ID      string `json:"id"`
			BibJSON struct {
				Title    string `json:"title"`
				Abstract string `json:"abstract"`
				Year     any    `json:"year"`
				Author   []struct {
					Name string `json:"name"`
				} `json:"author"`
				Identifier []struct {
					Type string `json:"type"`
					ID   string `json:"id"`
				} `json:"identifier"`
				Link []struct {
					Type string `json:"type"`
					URL  string `json:"url"`
				} `json:"link"`
				Journal struct {
					Title string `json:"title"`
				} `json:"journal"`
			} `json:"bibjson"`
		} `json:"results"`
	}
	values := url.Values{}
	values.Set("pageSize", strconv.Itoa(query.Limit))
	if err := p.getJSON(ctx, "/api/search/articles/"+url.PathEscape(query.Text), values, &resp); err != nil {
		return nil, err
	}
	candidates := make([]contracts.ReferenceCandidate, 0, len(resp.Results))
	for _, item := range resp.Results {
		doi := doajIdentifier(item.BibJSON.Identifier, "doi")
		accessURL := doajBestLink(item.BibJSON.Link)
		candidate := contracts.ReferenceCandidate{
			Title:        strings.TrimSpace(item.BibJSON.Title),
			Authors:      namesFromDOAJAuthors(item.BibJSON.Author),
			Year:         yearFromAny(item.BibJSON.Year),
			Source:       p.Name(),
			DOI:          doi,
			URL:          accessURL,
			Abstract:     cleanAbstract(item.BibJSON.Abstract),
			Venue:        item.BibJSON.Journal.Title,
			Reliability:  "official_api",
			Availability: "open_access",
			AccessURL:    accessURL,
			SourceID:     item.ID,
		}
		if candidate.URL == "" && candidate.DOI != "" {
			candidate.URL = "https://doi.org/" + references.NormalizeDOI(candidate.DOI)
		}
		if err := validateCandidate(candidate); err != nil {
			return nil, ProviderError{Code: CodeSearchFieldMissing, Source: p.Name(), Message: err.Error(), Retryable: false}
		}
		candidates = append(candidates, withStatusAndDedupe(candidate))
	}
	return candidates, nil
}

func namesFromDOAJAuthors(authors []struct {
	Name string `json:"name"`
}) []string {
	out := make([]string, 0, len(authors))
	for _, author := range authors {
		if name := strings.TrimSpace(author.Name); name != "" {
			out = append(out, name)
		}
	}
	return out
}

func doajIdentifier(identifiers []struct {
	Type string `json:"type"`
	ID   string `json:"id"`
}, typ string) string {
	for _, identifier := range identifiers {
		if strings.EqualFold(strings.TrimSpace(identifier.Type), typ) && strings.TrimSpace(identifier.ID) != "" {
			return strings.TrimSpace(identifier.ID)
		}
	}
	return ""
}

func doajBestLink(links []struct {
	Type string `json:"type"`
	URL  string `json:"url"`
}) string {
	for _, want := range []string{"fulltext", "oa", "landing_page"} {
		for _, link := range links {
			if strings.Contains(strings.ToLower(link.Type), want) && strings.TrimSpace(link.URL) != "" {
				return strings.TrimSpace(link.URL)
			}
		}
	}
	for _, link := range links {
		if strings.TrimSpace(link.URL) != "" {
			return strings.TrimSpace(link.URL)
		}
	}
	return ""
}

func yearFromAny(value any) int {
	switch v := value.(type) {
	case float64:
		return int(v)
	case int:
		return v
	case string:
		return yearFromString(v)
	default:
		return 0
	}
}
