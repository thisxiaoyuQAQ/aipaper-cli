package search

import (
	"context"
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"

	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/contracts"
)

const openAlexBaseURL = "https://api.openalex.org"

type OpenAlexProvider struct {
	httpProvider
}

func NewOpenAlexProvider(cfg HTTPProviderConfig) OpenAlexProvider {
	baseURL := cfg.BaseURL
	if baseURL == "" {
		baseURL = openAlexBaseURL
	}
	return OpenAlexProvider{httpProvider: httpProvider{
		name:    OpenAlexProviderName,
		baseURL: baseURL,
		client:  defaultHTTPClient(cfg.Client),
	}}
}

func (p OpenAlexProvider) Search(ctx context.Context, query Query) ([]contracts.ReferenceCandidate, error) {
	var resp struct {
		Results []struct {
			ID                  string           `json:"id"`
			DisplayName         string           `json:"display_name"`
			PublicationYear     int              `json:"publication_year"`
			DOI                 string           `json:"doi"`
			CitedByCount        int              `json:"cited_by_count"`
			AbstractInvertedIdx map[string][]int `json:"abstract_inverted_index"`
			Authorships         []struct {
				Author struct {
					DisplayName string `json:"display_name"`
				} `json:"author"`
			} `json:"authorships"`
			PrimaryLocation struct {
				LandingPageURL string `json:"landing_page_url"`
				Source         struct {
					DisplayName string `json:"display_name"`
				} `json:"source"`
			} `json:"primary_location"`
			OpenAccess struct {
				IsOA  bool   `json:"is_oa"`
				OAURL string `json:"oa_url"`
			} `json:"open_access"`
			HostVenue struct {
				DisplayName string `json:"display_name"`
			} `json:"host_venue"`
		} `json:"results"`
	}
	values := url.Values{}
	values.Set("search", query.Text)
	values.Set("per-page", strconv.Itoa(query.Limit))
	if err := p.getJSON(ctx, "/works", values, &resp); err != nil {
		return nil, err
	}
	candidates := make([]contracts.ReferenceCandidate, 0, len(resp.Results))
	for _, item := range resp.Results {
		availability := "landing_page"
		accessURL := strings.TrimSpace(item.PrimaryLocation.LandingPageURL)
		if item.OpenAccess.IsOA {
			availability = "open_access"
			if strings.TrimSpace(item.OpenAccess.OAURL) != "" {
				accessURL = strings.TrimSpace(item.OpenAccess.OAURL)
			}
		} else if strings.TrimSpace(item.DOI) != "" {
			availability = "doi_landing"
		}
		candidate := contracts.ReferenceCandidate{
			Title:         item.DisplayName,
			Authors:       namesFromOpenAlexAuthorships(item.Authorships),
			Year:          item.PublicationYear,
			Source:        p.Name(),
			DOI:           item.DOI,
			URL:           firstString([]string{accessURL, item.PrimaryLocation.LandingPageURL, item.DOI, item.ID}),
			Abstract:      abstractFromOpenAlexIndex(item.AbstractInvertedIdx),
			Venue:         firstString([]string{item.PrimaryLocation.Source.DisplayName, item.HostVenue.DisplayName}),
			CitationCount: item.CitedByCount,
			Reliability:   "official_api",
			Availability:  availability,
			AccessURL:     accessURL,
			SourceID:      item.ID,
		}
		if err := validateCandidate(candidate); err != nil {
			return nil, ProviderError{Code: CodeSearchFieldMissing, Source: p.Name(), Message: err.Error(), Retryable: false}
		}
		candidates = append(candidates, withStatusAndDedupe(candidate))
	}
	return candidates, nil
}

func namesFromOpenAlexAuthorships(authorships []struct {
	Author struct {
		DisplayName string `json:"display_name"`
	} `json:"author"`
}) []string {
	out := make([]string, 0, len(authorships))
	for _, authorship := range authorships {
		if name := strings.TrimSpace(authorship.Author.DisplayName); name != "" {
			out = append(out, name)
		}
	}
	return out
}

func abstractFromOpenAlexIndex(index map[string][]int) string {
	if len(index) == 0 {
		return ""
	}
	tokens := make([]string, 0, len(index))
	for word, positions := range index {
		for _, pos := range positions {
			tokens = append(tokens, fmt.Sprintf("%09d\x00%s", pos, word))
		}
	}
	sort.Strings(tokens)
	words := make([]string, 0, len(tokens))
	for _, token := range tokens {
		_, word, ok := strings.Cut(token, "\x00")
		if ok {
			words = append(words, word)
		}
	}
	return strings.Join(words, " ")
}
