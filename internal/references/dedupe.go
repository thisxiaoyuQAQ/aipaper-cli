package references

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/url"
	"sort"
	"strings"
	"unicode"

	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/contracts"
)

func AssignCandidateIDs(candidates []contracts.ReferenceCandidate, startIndex int) []contracts.ReferenceCandidate {
	if startIndex <= 0 {
		startIndex = 1
	}
	out := make([]contracts.ReferenceCandidate, len(candidates))
	for i, candidate := range candidates {
		candidate.ID = fmt.Sprintf("cand_%03d", startIndex+i)
		out[i] = candidate
	}
	return out
}

func DedupeCandidates(candidates []contracts.ReferenceCandidate) []contracts.ReferenceCandidate {
	type grouped struct {
		key       string
		firstSeen int
		best      contracts.ReferenceCandidate
	}
	groups := map[string]*grouped{}
	for i, candidate := range candidates {
		key := CandidateDedupeGroup(candidate)
		candidate.DedupeGroup = key
		if existing, ok := groups[key]; ok {
			if candidateCompleteness(candidate) > candidateCompleteness(existing.best) {
				existing.best = mergeCandidate(existing.best, candidate)
			} else {
				existing.best = mergeCandidate(candidate, existing.best)
			}
			continue
		}
		groups[key] = &grouped{key: key, firstSeen: i, best: candidate}
	}

	ordered := make([]*grouped, 0, len(groups))
	for _, group := range groups {
		ordered = append(ordered, group)
	}
	sort.SliceStable(ordered, func(i, j int) bool {
		return ordered[i].firstSeen < ordered[j].firstSeen
	})

	out := make([]contracts.ReferenceCandidate, 0, len(ordered))
	for _, group := range ordered {
		out = append(out, group.best)
	}
	return out
}

func CandidateDedupeGroup(candidate contracts.ReferenceCandidate) string {
	if doi := NormalizeDOI(candidate.DOI); doi != "" {
		return "doi:" + doi
	}
	if normalizedURL := NormalizeURL(candidate.URL); normalizedURL != "" {
		return "url:" + normalizedURL
	}
	title := normalizeText(candidate.Title)
	author := ""
	if len(candidate.Authors) > 0 {
		author = normalizeText(candidate.Authors[0])
	}
	fingerprint := fmt.Sprintf("%s|%s|%d", title, author, candidate.Year)
	sum := sha256.Sum256([]byte(fingerprint))
	return "hash:" + hex.EncodeToString(sum[:])[:16]
}

func NormalizeDOI(doi string) string {
	doi = strings.TrimSpace(strings.ToLower(doi))
	doi = strings.TrimPrefix(doi, "https://doi.org/")
	doi = strings.TrimPrefix(doi, "http://doi.org/")
	doi = strings.TrimPrefix(doi, "doi:")
	return strings.Trim(doi, ". ")
}

func NormalizeURL(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return ""
	}
	parsed.Scheme = strings.ToLower(parsed.Scheme)
	parsed.Host = strings.ToLower(parsed.Host)
	parsed.Fragment = ""
	if parsed.Path == "/" {
		parsed.Path = ""
	}
	return strings.TrimRight(parsed.String(), "/")
}

func mergeCandidate(lower, higher contracts.ReferenceCandidate) contracts.ReferenceCandidate {
	out := higher
	if out.Title == "" {
		out.Title = lower.Title
	}
	if len(out.Authors) == 0 {
		out.Authors = lower.Authors
	}
	if out.Year == 0 {
		out.Year = lower.Year
	}
	if out.Source == "" {
		out.Source = lower.Source
	}
	if out.DOI == "" {
		out.DOI = lower.DOI
	}
	if out.URL == "" {
		out.URL = lower.URL
	}
	if out.Abstract == "" {
		out.Abstract = lower.Abstract
	}
	if out.Venue == "" {
		out.Venue = lower.Venue
	}
	if out.CitationCount == 0 {
		out.CitationCount = lower.CitationCount
	}
	if out.Reliability == "" {
		out.Reliability = lower.Reliability
	}
	if out.Availability == "" {
		out.Availability = lower.Availability
	}
	if out.AccessURL == "" {
		out.AccessURL = lower.AccessURL
	}
	if out.SourceID == "" {
		out.SourceID = lower.SourceID
	}
	if out.RelevanceScore == 0 {
		out.RelevanceScore = lower.RelevanceScore
	}
	if out.RelevanceReason == "" {
		out.RelevanceReason = lower.RelevanceReason
	}
	if out.Status == "" {
		out.Status = lower.Status
	}
	out.DedupeGroup = CandidateDedupeGroup(out)
	return out
}

func candidateCompleteness(candidate contracts.ReferenceCandidate) int {
	score := 0
	if candidate.Title != "" {
		score += 4
	}
	if len(candidate.Authors) > 0 {
		score += 2
	}
	if candidate.Year != 0 {
		score++
	}
	if candidate.DOI != "" {
		score += 3
	}
	if candidate.URL != "" {
		score++
	}
	if candidate.Abstract != "" {
		score += 2
	}
	if candidate.Venue != "" {
		score++
	}
	if candidate.CitationCount != 0 {
		score++
	}
	if candidate.Reliability != "" {
		score++
	}
	if candidate.Availability != "" {
		score++
	}
	if candidate.AccessURL != "" {
		score++
	}
	if candidate.RelevanceScore != 0 {
		score++
	}
	return score
}

func normalizeText(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	var b strings.Builder
	lastSpace := false
	for _, r := range value {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
			lastSpace = false
			continue
		}
		if !lastSpace {
			b.WriteByte(' ')
			lastSpace = true
		}
	}
	return strings.TrimSpace(b.String())
}
