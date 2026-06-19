package search

import (
	"context"
	"encoding/xml"
	"net/url"
	"strconv"
	"strings"

	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/contracts"
)

const arxivBaseURL = "https://export.arxiv.org"

type ArxivProvider struct {
	httpProvider
}

func NewArxivProvider(cfg HTTPProviderConfig) ArxivProvider {
	baseURL := cfg.BaseURL
	if baseURL == "" {
		baseURL = arxivBaseURL
	}
	return ArxivProvider{httpProvider: httpProvider{
		name:    ArxivProviderName,
		baseURL: baseURL,
		client:  defaultHTTPClient(cfg.Client),
	}}
}

func (p ArxivProvider) Search(ctx context.Context, query Query) ([]contracts.ReferenceCandidate, error) {
	values := url.Values{}
	values.Set("search_query", "all:"+query.Text)
	values.Set("start", "0")
	values.Set("max_results", strconv.Itoa(query.Limit))
	data, err := p.getText(ctx, "/api/query", values)
	if err != nil {
		return nil, err
	}
	var feed arxivFeed
	if err := xml.Unmarshal(data, &feed); err != nil {
		return nil, err
	}
	candidates := make([]contracts.ReferenceCandidate, 0, len(feed.Entries))
	for _, entry := range feed.Entries {
		candidate := contracts.ReferenceCandidate{
			Title:        cleanXMLText(entry.Title),
			Authors:      entry.AuthorNames(),
			Year:         yearFromString(entry.Published),
			Source:       p.Name(),
			DOI:          entry.DOI,
			URL:          entry.URL(),
			Abstract:     cleanXMLText(entry.Summary),
			Venue:        "arXiv",
			Reliability:  "repository",
			Availability: "open_access",
			AccessURL:    entry.URL(),
		}
		if err := validateCandidate(candidate); err != nil {
			return nil, ProviderError{Code: CodeSearchFieldMissing, Source: p.Name(), Message: err.Error(), Retryable: false}
		}
		candidates = append(candidates, withStatusAndDedupe(candidate))
	}
	return candidates, nil
}

type arxivFeed struct {
	Entries []arxivEntry `xml:"entry"`
}

type arxivEntry struct {
	ID        string        `xml:"id"`
	Title     string        `xml:"title"`
	Summary   string        `xml:"summary"`
	Published string        `xml:"published"`
	DOI       string        `xml:"doi"`
	Authors   []arxivAuthor `xml:"author"`
	Links     []arxivLink   `xml:"link"`
}

type arxivAuthor struct {
	Name string `xml:"name"`
}

type arxivLink struct {
	Href string `xml:"href,attr"`
	Rel  string `xml:"rel,attr"`
	Type string `xml:"type,attr"`
}

func (e arxivEntry) AuthorNames() []string {
	names := make([]string, 0, len(e.Authors))
	for _, author := range e.Authors {
		if strings.TrimSpace(author.Name) != "" {
			names = append(names, strings.TrimSpace(author.Name))
		}
	}
	return names
}

func (e arxivEntry) URL() string {
	for _, link := range e.Links {
		if link.Rel == "alternate" && strings.TrimSpace(link.Href) != "" {
			return strings.TrimSpace(link.Href)
		}
	}
	if strings.TrimSpace(e.ID) != "" {
		return strings.TrimSpace(e.ID)
	}
	return ""
}

func cleanXMLText(value string) string {
	return strings.Join(strings.Fields(value), " ")
}
