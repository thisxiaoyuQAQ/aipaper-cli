package search

import (
	"context"
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
	rateLimitedProviders := map[string]bool{}
	for _, provider := range providers {
		candidates, providerErrors := searchProvider(ctx, provider, query, "")
		all = append(all, candidates...)
		result.Errors = appendUniqueProviderErrors(result.Errors, providerErrors)
		markRateLimitedProviders(rateLimitedProviders, providerErrors)
	}
	all, result.Errors = expandCandidates(ctx, all, query, providers, opts, result.Errors, rateLimitedProviders)

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

func searchProvider(ctx context.Context, provider Provider, query Query, expansionSource string) ([]contracts.ReferenceCandidate, []ProviderError) {
	candidates, err := provider.Search(ctx, query)
	if err != nil {
		return nil, []ProviderError{normalizeProviderError(provider.Name(), err)}
	}
	var out []contracts.ReferenceCandidate
	var errors []ProviderError
	for _, candidate := range candidates {
		if candidate.Status == "" {
			candidate.Status = "pending"
		}
		if candidate.Source == "" {
			candidate.Source = provider.Name()
		}
		if expansionSource != "" {
			candidate.ExpansionSource = expansionSource
			if strings.TrimSpace(candidate.RelevanceReason) == "" {
				candidate.RelevanceReason = "Expanded from query: " + expansionSource
			}
		}
		if err := validateCandidate(candidate); err != nil {
			errors = append(errors, ProviderError{
				Code:      CodeSearchFieldMissing,
				Source:    provider.Name(),
				Message:   err.Error(),
				Retryable: false,
			})
			continue
		}
		out = append(out, candidate)
	}
	return out, errors
}

func expandCandidates(ctx context.Context, current []contracts.ReferenceCandidate, base Query, providers []Provider, opts Options, errors []ProviderError, rateLimitedProviders map[string]bool) ([]contracts.ReferenceCandidate, []ProviderError) {
	if !opts.ExpansionEnabled {
		return current, errors
	}
	minCandidates := opts.MinCandidateCount
	if minCandidates <= 0 {
		minCandidates = opts.Limit
	}
	if minCandidates <= 0 {
		minCandidates = 10
	}
	if len(references.DedupeCandidates(current)) >= minCandidates {
		return current, errors
	}
	expansionLimit := opts.ExpansionLimit
	if expansionLimit <= 0 {
		expansionLimit = 3
	}
	queries := ExpansionQueriesFromRequirements(opts.Requirements, base, expansionLimit)
	for _, expansionQuery := range queries {
		for _, provider := range providers {
			if rateLimitedProviders[strings.ToLower(provider.Name())] {
				continue
			}
			candidates, providerErrors := searchProvider(ctx, provider, expansionQuery, expansionQuery.Text)
			errors = appendUniqueProviderErrors(errors, providerErrors)
			markRateLimitedProviders(rateLimitedProviders, providerErrors)
			current = append(current, candidates...)
			if len(references.DedupeCandidates(current)) >= minCandidates {
				return current, errors
			}
		}
	}
	return current, errors
}

func appendUniqueProviderErrors(existing []ProviderError, additions []ProviderError) []ProviderError {
	for _, err := range additions {
		key := providerErrorKey(err)
		duplicate := false
		for _, current := range existing {
			if providerErrorKey(current) == key {
				duplicate = true
				break
			}
		}
		if !duplicate {
			existing = append(existing, err)
		}
	}
	return existing
}

func providerErrorKey(err ProviderError) string {
	return strings.ToLower(strings.TrimSpace(err.Source)) + "\x00" + strings.TrimSpace(err.Code) + "\x00" + strings.TrimSpace(err.Message)
}

func markRateLimitedProviders(skip map[string]bool, errors []ProviderError) {
	if skip == nil {
		return
	}
	for _, err := range errors {
		if err.Code == CodeSearchRateLimited && strings.TrimSpace(err.Source) != "" {
			skip[strings.ToLower(strings.TrimSpace(err.Source))] = true
		}
	}
}

func writeResult(s store.Store, result Result) (Result, error) {
	outputs, err := references.WriteCandidates(s, result.Candidates.Items)
	if err != nil {
		return result, err
	}
	result.Outputs = outputs
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

func CandidateJSONPath(s store.Store) string {
	return s.Path(filepath.FromSlash("references/candidates.json"))
}
