package search

import (
	"html"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/contracts"
	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/references"
)

const (
	OpenAlexProviderName        = "openalex"
	SemanticScholarProviderName = "semantic_scholar"
	CrossrefProviderName        = "crossref"
	ArxivProviderName           = "arxiv"
	PubMedProviderName          = "pubmed"
	DOAJProviderName            = "doaj"
)

func DefaultProviders(cfg HTTPProviderConfig) []Provider {
	client := defaultHTTPClient(cfg.Client)
	return []Provider{
		NewOpenAlexProvider(HTTPProviderConfig{Client: client}),
		NewCrossrefProvider(HTTPProviderConfig{Client: client}),
		NewSemanticScholarProvider(HTTPProviderConfig{Client: client}),
		NewArxivProvider(HTTPProviderConfig{Client: client}),
		NewPubMedProvider(HTTPProviderConfig{Client: client}),
		NewDOAJProvider(HTTPProviderConfig{Client: client}),
	}
}

func candidateAvailability(urlValue, doi string) string {
	if strings.TrimSpace(urlValue) != "" {
		return "landing_page"
	}
	if strings.TrimSpace(doi) != "" {
		return "doi_landing"
	}
	return "unknown"
}

func withStatusAndDedupe(candidate contracts.ReferenceCandidate) contracts.ReferenceCandidate {
	if candidate.Status == "" {
		candidate.Status = "pending"
	}
	candidate.DOI = references.NormalizeDOI(candidate.DOI)
	if candidate.DedupeGroup == "" {
		candidate.DedupeGroup = references.CandidateDedupeGroup(candidate)
	}
	return candidate
}

func namesFromSemanticAuthors(authors []struct {
	Name string `json:"name"`
}) []string {
	out := make([]string, 0, len(authors))
	for _, author := range authors {
		if strings.TrimSpace(author.Name) != "" {
			out = append(out, strings.TrimSpace(author.Name))
		}
	}
	return out
}

func firstString(values []string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func yearFromDateParts(parts [][]int) int {
	if len(parts) == 0 || len(parts[0]) == 0 {
		return 0
	}
	return parts[0][0]
}

func yearFromString(value string) int {
	for i := 0; i+4 <= len(value); i++ {
		year, err := strconv.Atoi(value[i : i+4])
		if err == nil && year >= 1000 && year <= time.Now().UTC().Year()+1 {
			return year
		}
	}
	return 0
}

var htmlTagRE = regexp.MustCompile(`<[^>]+>`)

func cleanAbstract(value string) string {
	value = htmlTagRE.ReplaceAllString(value, " ")
	value = html.UnescapeString(value)
	return strings.Join(strings.Fields(value), " ")
}
