package search

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/contracts"
	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/references"
	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/store"
)

func Run(ctx context.Context, s store.Store, opts Options) (Result, error) {
	if err := store.EnsureLayout(s); err != nil {
		return Result{}, err
	}
	query := QueryFromRequirements(opts.Requirements, opts.Limit)
	providers := opts.Providers
	if len(providers) == 0 {
		providers = DefaultProviders(HTTPProviderConfig{Client: opts.HTTPClient})
	}
	providers = filterProviders(providers, opts.Requirements.SearchProviders)

	result := Result{Candidates: contracts.ReferenceCandidates{Items: []contracts.ReferenceCandidate{}}}
	if !opts.Requirements.AllowOnlineSearch {
		return writeResult(s, result)
	}
	if strings.TrimSpace(query.Text) == "" {
		result.Errors = append(result.Errors, ProviderError{
			Code:      CodeSearchFieldMissing,
			Message:   "search query is empty",
			Retryable: false,
		})
		return writeResult(s, result)
	}
	if len(providers) == 0 {
		result.Errors = append(result.Errors, ProviderError{
			Code:      CodeProviderUnsupported,
			Message:   "no configured search providers are available",
			Retryable: false,
		})
		return writeResult(s, result)
	}

	var all []contracts.ReferenceCandidate
	for _, provider := range providers {
		candidates, err := provider.Search(ctx, query)
		if err != nil {
			result.Errors = append(result.Errors, normalizeProviderError(provider.Name(), err))
			continue
		}
		for _, candidate := range candidates {
			if candidate.Status == "" {
				candidate.Status = "pending"
			}
			if candidate.Source == "" {
				candidate.Source = provider.Name()
			}
			if err := validateCandidate(candidate); err != nil {
				result.Errors = append(result.Errors, ProviderError{
					Code:      CodeSearchFieldMissing,
					Source:    provider.Name(),
					Message:   err.Error(),
					Retryable: false,
				})
				continue
			}
			all = append(all, candidate)
		}
	}

	deduped := references.DedupeCandidates(all)
	result.Candidates.Items = references.AssignCandidateIDs(deduped, 1)
	if len(result.Candidates.Items) == 0 && len(result.Errors) == 0 {
		result.Errors = append(result.Errors, ProviderError{
			Code:      CodeCandidatesEmpty,
			Message:   "no reference candidates returned",
			Retryable: false,
		})
	}
	return writeResult(s, result)
}

func writeResult(s store.Store, result Result) (Result, error) {
	jsonPath := s.Path("references", "candidates.json")
	mdPath := s.Path("references", "candidates.md")
	if res, err := store.WriteJSON(jsonPath, result.Candidates, store.Overwrite); err != nil {
		return result, err
	} else {
		result.Outputs = append(result.Outputs, res.Path)
	}
	if res, err := store.WriteFile(mdPath, []byte(formatCandidatesMarkdown(result.Candidates.Items)), store.Overwrite); err != nil {
		return result, err
	} else {
		result.Outputs = append(result.Outputs, res.Path)
	}
	return result, nil
}

func filterProviders(providers []Provider, names []string) []Provider {
	if len(names) == 0 {
		return providers
	}
	byName := map[string]Provider{}
	for _, provider := range providers {
		byName[strings.ToLower(provider.Name())] = provider
	}
	var filtered []Provider
	seen := map[string]bool{}
	for _, name := range names {
		key := strings.ToLower(strings.TrimSpace(name))
		if key == "" || seen[key] {
			continue
		}
		if provider, ok := byName[key]; ok {
			filtered = append(filtered, provider)
			seen[key] = true
		}
	}
	return filtered
}

func formatCandidatesMarkdown(candidates []contracts.ReferenceCandidate) string {
	var b strings.Builder
	b.WriteString("# Reference Candidates\n\n")
	if len(candidates) == 0 {
		b.WriteString("No reference candidates.\n")
		return b.String()
	}
	for _, candidate := range candidates {
		fmt.Fprintf(&b, "## %s\n\n", candidate.ID)
		fmt.Fprintf(&b, "- Title: %s\n", candidate.Title)
		fmt.Fprintf(&b, "- Authors: %s\n", strings.Join(candidate.Authors, ", "))
		if candidate.Year != 0 {
			fmt.Fprintf(&b, "- Year: %d\n", candidate.Year)
		}
		fmt.Fprintf(&b, "- Source: %s\n", candidate.Source)
		if candidate.DOI != "" {
			fmt.Fprintf(&b, "- DOI: %s\n", candidate.DOI)
		}
		if candidate.URL != "" {
			fmt.Fprintf(&b, "- URL: %s\n", candidate.URL)
		}
		if candidate.Venue != "" {
			fmt.Fprintf(&b, "- Venue: %s\n", candidate.Venue)
		}
		if candidate.CitationCount != 0 {
			fmt.Fprintf(&b, "- Citation count: %d\n", candidate.CitationCount)
		}
		if candidate.DedupeGroup != "" {
			fmt.Fprintf(&b, "- Dedupe group: %s\n", candidate.DedupeGroup)
		}
		if candidate.Abstract != "" {
			fmt.Fprintf(&b, "\n%s\n", candidate.Abstract)
		}
		b.WriteString("\n")
	}
	return b.String()
}

func CandidateJSONPath(s store.Store) string {
	return s.Path(filepath.FromSlash("references/candidates.json"))
}
