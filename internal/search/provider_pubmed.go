package search

import (
	"context"
	"encoding/json"
	"net/url"
	"strconv"
	"strings"

	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/contracts"
)

const pubMedBaseURL = "https://eutils.ncbi.nlm.nih.gov/entrez/eutils"

type PubMedProvider struct {
	httpProvider
}

func NewPubMedProvider(cfg HTTPProviderConfig) PubMedProvider {
	baseURL := cfg.BaseURL
	if baseURL == "" {
		baseURL = pubMedBaseURL
	}
	return PubMedProvider{httpProvider: httpProvider{
		name:    PubMedProviderName,
		baseURL: baseURL,
		client:  defaultHTTPClient(cfg.Client),
	}}
}

func (p PubMedProvider) Search(ctx context.Context, query Query) ([]contracts.ReferenceCandidate, error) {
	var searchResp struct {
		ESearchResult struct {
			IDList []string `json:"idlist"`
		} `json:"esearchresult"`
	}
	searchValues := url.Values{}
	searchValues.Set("db", "pubmed")
	searchValues.Set("term", query.Text)
	searchValues.Set("retmode", "json")
	searchValues.Set("retmax", strconv.Itoa(query.Limit))
	if err := p.getJSON(ctx, "/esearch.fcgi", searchValues, &searchResp); err != nil {
		return nil, err
	}
	if len(searchResp.ESearchResult.IDList) == 0 {
		return []contracts.ReferenceCandidate{}, nil
	}

	var summaryResp pubMedSummaryResponse
	summaryValues := url.Values{}
	summaryValues.Set("db", "pubmed")
	summaryValues.Set("id", strings.Join(searchResp.ESearchResult.IDList, ","))
	summaryValues.Set("retmode", "json")
	summaryValues.Set("version", "2.0")
	if err := p.getJSON(ctx, "/esummary.fcgi", summaryValues, &summaryResp); err != nil {
		return nil, err
	}

	candidates := make([]contracts.ReferenceCandidate, 0, len(summaryResp.Result.UIDs))
	for _, uid := range summaryResp.Result.UIDs {
		item, ok := summaryResp.Result.Items[uid]
		if !ok {
			continue
		}
		candidate := contracts.ReferenceCandidate{
			Title:         item.Title,
			Authors:       item.AuthorNames(),
			Year:          yearFromString(item.PubDate),
			Source:        p.Name(),
			DOI:           item.DOI(),
			URL:           "https://pubmed.ncbi.nlm.nih.gov/" + uid + "/",
			Venue:         item.FullJournalName,
			CitationCount: 0,
			Reliability:   "official_api",
			Availability:  "landing_page",
			AccessURL:     "https://pubmed.ncbi.nlm.nih.gov/" + uid + "/",
			SourceID:      uid,
		}
		if err := validateCandidate(candidate); err != nil {
			return nil, ProviderError{Code: CodeSearchFieldMissing, Source: p.Name(), Message: err.Error(), Retryable: false}
		}
		candidates = append(candidates, withStatusAndDedupe(candidate))
	}
	return candidates, nil
}

type pubMedSummaryResponse struct {
	Result struct {
		UIDs  []string                     `json:"uids"`
		Items map[string]pubMedSummaryItem `json:"-"`
	} `json:"result"`
}

type pubMedSummaryItem struct {
	UID             string `json:"uid"`
	Title           string `json:"title"`
	FullJournalName string `json:"fulljournalname"`
	PubDate         string `json:"pubdate"`
	ELocationID     string `json:"elocationid"`
	Authors         []struct {
		Name string `json:"name"`
	} `json:"authors"`
	ArticleIDs []struct {
		IDType string `json:"idtype"`
		Value  string `json:"value"`
	} `json:"articleids"`
}

func (r *pubMedSummaryResponse) UnmarshalJSON(data []byte) error {
	type rawItem pubMedSummaryItem
	var raw struct {
		Result map[string]json.RawMessage `json:"result"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	r.Result.Items = map[string]pubMedSummaryItem{}
	for key, value := range raw.Result {
		if key == "uids" {
			if err := json.Unmarshal(value, &r.Result.UIDs); err != nil {
				return err
			}
			continue
		}
		var item rawItem
		if err := json.Unmarshal(value, &item); err != nil {
			return err
		}
		r.Result.Items[key] = pubMedSummaryItem(item)
	}
	return nil
}

func (i pubMedSummaryItem) AuthorNames() []string {
	names := make([]string, 0, len(i.Authors))
	for _, author := range i.Authors {
		if strings.TrimSpace(author.Name) != "" {
			names = append(names, strings.TrimSpace(author.Name))
		}
	}
	return names
}

func (i pubMedSummaryItem) DOI() string {
	for _, id := range i.ArticleIDs {
		if strings.EqualFold(id.IDType, "doi") && strings.TrimSpace(id.Value) != "" {
			return strings.TrimSpace(id.Value)
		}
	}
	if strings.Contains(strings.ToLower(i.ELocationID), "doi") {
		return strings.TrimSpace(strings.TrimPrefix(i.ELocationID, "doi:"))
	}
	return ""
}
