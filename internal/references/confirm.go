package references

import (
	"errors"
	"fmt"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode"

	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/contracts"
	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/store"
)

const CodeNoneConfirmed = "REFERENCE_NONE_CONFIRMED"

type ConfirmError struct {
	Code    string
	Message string
}

func (e ConfirmError) Error() string {
	return e.Code + ": " + e.Message
}

type ConfirmationDecision struct {
	ConfirmedIDs []string
	RejectedIDs  []string
	ConfirmedAt  time.Time
}

type ConfirmationResult struct {
	Confirmed contracts.ConfirmedReferences
	Rejected  contracts.ReferenceCandidates
	Outputs   []string
}

func LoadCandidates(s store.Store) (contracts.ReferenceCandidates, error) {
	var candidates contracts.ReferenceCandidates
	if err := store.ReadJSON(s.Path("references", "candidates.json"), &candidates); err != nil {
		return contracts.ReferenceCandidates{}, err
	}
	return candidates, nil
}

func ConfirmCandidates(s store.Store, candidates contracts.ReferenceCandidates, decision ConfirmationDecision) (ConfirmationResult, error) {
	if len(decision.ConfirmedIDs) == 0 {
		return ConfirmationResult{}, ConfirmError{Code: CodeNoneConfirmed, Message: "at least one reference must be confirmed"}
	}
	if decision.ConfirmedAt.IsZero() {
		decision.ConfirmedAt = time.Now().UTC()
	}
	if err := store.EnsureLayout(s); err != nil {
		return ConfirmationResult{}, err
	}

	byID := map[string]contracts.ReferenceCandidate{}
	for _, candidate := range candidates.Items {
		byID[candidate.ID] = candidate
	}
	confirmedSet := idSet(decision.ConfirmedIDs)
	rejectedSet := idSet(decision.RejectedIDs)
	usedKeys := map[string]int{}
	result := ConfirmationResult{
		Confirmed: contracts.ConfirmedReferences{Items: []contracts.ConfirmedReference{}},
		Rejected:  contracts.ReferenceCandidates{Items: []contracts.ReferenceCandidate{}},
	}

	for _, id := range decision.ConfirmedIDs {
		candidate, ok := byID[id]
		if !ok {
			return result, fmt.Errorf("confirmed candidate %q not found", id)
		}
		key := UniqueReferenceKey(candidate, usedKeys)
		result.Confirmed.Items = append(result.Confirmed.Items, contracts.ConfirmedReference{
			Key:               key,
			Title:             candidate.Title,
			Authors:           append([]string(nil), candidate.Authors...),
			Year:              candidate.Year,
			DOI:               candidate.DOI,
			URL:               candidate.URL,
			Abstract:          candidate.Abstract,
			Venue:             candidate.Venue,
			Reliability:       candidate.Reliability,
			Availability:      candidate.Availability,
			AccessURL:         candidate.AccessURL,
			SourceID:          candidate.SourceID,
			SourceMaterialIDs: []string{},
			ConfirmedAt:       decision.ConfirmedAt,
		})
	}
	for _, candidate := range candidates.Items {
		if confirmedSet[candidate.ID] {
			continue
		}
		if len(rejectedSet) > 0 && !rejectedSet[candidate.ID] {
			continue
		}
		candidate.Status = "rejected"
		result.Rejected.Items = append(result.Rejected.Items, candidate)
	}
	if err := writeConfirmation(s, &result); err != nil {
		return result, err
	}
	return result, nil
}

func writeConfirmation(s store.Store, result *ConfirmationResult) error {
	if res, err := store.WriteJSON(s.Path("references", "confirmed.json"), result.Confirmed, store.Overwrite); err != nil {
		return err
	} else {
		result.Outputs = append(result.Outputs, res.Path)
	}
	if res, err := store.WriteJSON(s.Path("references", "rejected.json"), result.Rejected, store.Overwrite); err != nil {
		return err
	} else {
		result.Outputs = append(result.Outputs, res.Path)
	}
	if res, err := store.WriteFile(s.Path(filepath.FromSlash("references/confirmed.bib")), []byte(FormatConfirmedBibTeX(result.Confirmed)), store.Overwrite); err != nil {
		return err
	} else {
		result.Outputs = append(result.Outputs, res.Path)
	}
	return nil
}

func UniqueReferenceKey(candidate contracts.ReferenceCandidate, used map[string]int) string {
	base := ReferenceKey(candidate)
	if base == "" {
		base = "ref"
	}
	count := used[base]
	used[base] = count + 1
	if count == 0 {
		return base
	}
	return base + suffixFor(count)
}

func ReferenceKey(candidate contracts.ReferenceCandidate) string {
	author := "unknown"
	if len(candidate.Authors) > 0 {
		author = surname(candidate.Authors[0])
	}
	author = lowerCamelToken(author)
	if author == "" {
		author = "unknown"
	}
	year := ""
	if candidate.Year != 0 {
		year = fmt.Sprintf("%d", candidate.Year)
	}
	title := shortTitle(candidate.Title)
	return author + year + title
}

func FormatConfirmedBibTeX(confirmed contracts.ConfirmedReferences) string {
	var b strings.Builder
	items := append([]contracts.ConfirmedReference(nil), confirmed.Items...)
	sort.SliceStable(items, func(i, j int) bool {
		return items[i].Key < items[j].Key
	})
	for _, ref := range items {
		entryType := "article"
		if ref.DOI == "" && ref.URL != "" {
			entryType = "misc"
		}
		fmt.Fprintf(&b, "@%s{%s,\n", entryType, ref.Key)
		writeBibField(&b, "title", ref.Title)
		if len(ref.Authors) > 0 {
			writeBibField(&b, "author", strings.Join(ref.Authors, " and "))
		}
		if ref.Year != 0 {
			writeBibField(&b, "year", fmt.Sprintf("%d", ref.Year))
		}
		writeBibField(&b, "doi", ref.DOI)
		writeBibField(&b, "url", ref.URL)
		writeBibField(&b, "journal", ref.Venue)
		writeBibField(&b, "abstract", ref.Abstract)
		b.WriteString("}\n\n")
	}
	return b.String()
}

func writeBibField(b *strings.Builder, name, value string) {
	if strings.TrimSpace(value) == "" {
		return
	}
	fmt.Fprintf(b, "  %s = {%s},\n", name, escapeBibTeX(value))
}

func escapeBibTeX(value string) string {
	value = strings.ReplaceAll(value, `\`, `\\`)
	value = strings.ReplaceAll(value, "{", `\{`)
	value = strings.ReplaceAll(value, "}", `\}`)
	return value
}

func idSet(ids []string) map[string]bool {
	out := map[string]bool{}
	for _, id := range ids {
		if strings.TrimSpace(id) != "" {
			out[strings.TrimSpace(id)] = true
		}
	}
	return out
}

func suffixFor(index int) string {
	if index <= 0 {
		return ""
	}
	var letters []byte
	for index > 0 {
		index--
		letters = append([]byte{byte('A' + index%26)}, letters...)
		index /= 26
	}
	return string(letters)
}

func surname(author string) string {
	author = strings.TrimSpace(author)
	if author == "" {
		return ""
	}
	if strings.Contains(author, ",") {
		return strings.TrimSpace(strings.Split(author, ",")[0])
	}
	parts := strings.Fields(author)
	if len(parts) == 0 {
		return ""
	}
	return parts[len(parts)-1]
}

var stopTitleWords = map[string]bool{
	"a": true, "an": true, "and": true, "for": true, "in": true, "of": true, "on": true, "the": true, "to": true, "with": true,
}

func shortTitle(title string) string {
	words := wordTokens(title)
	var picked []string
	for _, word := range words {
		lower := strings.ToLower(word)
		if stopTitleWords[lower] {
			continue
		}
		picked = append(picked, titleToken(word))
		if len(picked) == 3 {
			break
		}
	}
	if len(picked) == 0 {
		return ""
	}
	return strings.Join(picked, "")
}

func lowerCamelToken(value string) string {
	words := wordTokens(value)
	if len(words) == 0 {
		return ""
	}
	return strings.ToLower(words[0])
}

func titleToken(value string) string {
	value = strings.ToLower(value)
	runes := []rune(value)
	if len(runes) == 0 {
		return ""
	}
	runes[0] = unicode.ToUpper(runes[0])
	return string(runes)
}

var tokenRE = regexp.MustCompile(`[\p{L}\p{N}]+`)

func wordTokens(value string) []string {
	return tokenRE.FindAllString(value, -1)
}

func IsNoneConfirmed(err error) bool {
	var confirmErr ConfirmError
	return errors.As(err, &confirmErr) && confirmErr.Code == CodeNoneConfirmed
}
