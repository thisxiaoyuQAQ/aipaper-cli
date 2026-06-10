package quality

// Module 26: claim graph types, chapter-level incremental merge, cross-chapter
// duplicate detection, and atomic persistence. Existing validation semantics in
// evidence.go / sectionplan.go are reused through their public APIs only.

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/artifacts"
	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/contracts"
	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/store"
)

// Claim support levels. Empty means extraction is done but verification has
// not run yet; skipped is the fast-mode marker (verification intentionally
// not performed, hard machine checks still apply).
const (
	SupportSupported          = "supported"
	SupportPartiallySupported = "partially_supported"
	SupportUnsupported        = "unsupported"
	SupportOverstated         = "overstated"
	SupportSkipped            = "skipped"
)

// Claim risk levels. Empty means not graded yet.
const (
	RiskHigh   = "high"
	RiskMedium = "medium"
	RiskLow    = "low"
)

// Structured error codes for claim graph validation.
const (
	CodeClaimGraphInvalid              = "claim_graph_invalid"
	CodeClaimGraphDuplicateID          = "claim_graph_duplicate_id"
	CodeClaimGraphUnknownChapter       = "claim_graph_unknown_chapter"
	CodeClaimGraphUnconfirmedReference = "claim_graph_unconfirmed_reference"
	CodeClaimGraphUnknownEvidence      = "claim_graph_unknown_evidence"
	CodeClaimGraphIOFailed             = "claim_graph_io_failed"
)

// ClaimGraph is the post-writing claim verification graph persisted at
// quality/claim-graph.json. Chapters are merged incrementally: extracting a
// chapter replaces only that chapter's nodes. See docs/interfaces/quality.md.
type ClaimGraph struct {
	UpdatedAt time.Time   `json:"updated_at"`
	Claims    []ClaimNode `json:"claims"`
}

// ClaimNode is one structured claim extracted from a chapter draft.
type ClaimNode struct {
	ID               string   `json:"id"`                        // claim_001 style, unique across the graph
	SourceClaimID    string   `json:"source_claim_id,omitempty"` // claim id inside the chapter claims.json
	Text             string   `json:"text"`
	ChapterID        string   `json:"chapter_id"`     // must match an outline chapter
	ReferenceKeys    []string `json:"reference_keys"` // must exist in references/confirmed.json
	EvidenceIDs      []string `json:"evidence_ids"`   // must exist in the evidence table
	Support          string   `json:"support"`        // empty until verification; skipped in fast mode
	RiskLevel        string   `json:"risk_level"`     // high|medium|low, empty until graded
	VerifierNote     string   `json:"verifier_note,omitempty"`
	DuplicateOf      []string `json:"duplicate_of,omitempty"` // cross-chapter near-duplicate claim ids
	NeedsRewrite     bool     `json:"needs_rewrite"`
	NeedsHumanReview bool     `json:"needs_human_review"`
}

var claimIDPattern = regexp.MustCompile(`^claim_\d{3,}$`)

// ClaimGraphJSONRel / ClaimGraphMarkdownRel are paths relative to the store root.
const (
	ClaimGraphJSONRel     = "quality/claim-graph.json"
	ClaimGraphMarkdownRel = "quality/claim-graph.md"
)

// duplicateSimilarityThreshold is the token Jaccard similarity above which two
// claims in different chapters are flagged as duplicates. First version uses
// plain text normalization, no vector store.
const duplicateSimilarityThreshold = 0.8

// ClaimGraphJSONPath returns the absolute JSON path inside the store.
func ClaimGraphJSONPath(s store.Store) string {
	return s.Path(filepath.FromSlash(ClaimGraphJSONRel))
}

// ClaimGraphMarkdownPath returns the absolute Markdown path inside the store.
func ClaimGraphMarkdownPath(s store.Store) string {
	return s.Path(filepath.FromSlash(ClaimGraphMarkdownRel))
}

// SaveClaimGraph validates the graph against the outline, confirmed references,
// and the evidence table, then atomically writes JSON + Markdown.
// Returns the written paths relative to the store root (forward slashes).
func SaveClaimGraph(s store.Store, graph ClaimGraph) ([]string, error) {
	if err := ValidateClaimGraph(s, graph); err != nil {
		return nil, err
	}
	if _, err := store.WriteJSON(ClaimGraphJSONPath(s), graph, store.Overwrite); err != nil {
		return nil, NewError(CodeClaimGraphIOFailed, fmt.Sprintf("write %s: %v", ClaimGraphJSONRel, err), true)
	}
	md := FormatClaimGraphMarkdown(graph)
	if _, err := store.WriteFile(ClaimGraphMarkdownPath(s), []byte(md), store.Overwrite); err != nil {
		return nil, NewError(CodeClaimGraphIOFailed, fmt.Sprintf("write %s: %v", ClaimGraphMarkdownRel, err), true)
	}
	return []string{ClaimGraphJSONRel, ClaimGraphMarkdownRel}, nil
}

// LoadClaimGraph reads quality/claim-graph.json with strict JSON parsing.
// A missing file returns a structured error.
func LoadClaimGraph(s store.Store) (ClaimGraph, error) {
	var graph ClaimGraph
	err := store.ReadJSON(ClaimGraphJSONPath(s), &graph)
	if errors.Is(err, os.ErrNotExist) {
		return ClaimGraph{}, NewError(CodeClaimGraphIOFailed, "claim graph not found: "+ClaimGraphJSONRel, false)
	}
	if err != nil {
		return ClaimGraph{}, NewError(CodeClaimGraphIOFailed, fmt.Sprintf("read %s: %v", ClaimGraphJSONRel, err), false)
	}
	return graph, nil
}

// ValidateClaimGraph checks id style and uniqueness, chapter ids against the
// outline, reference keys against confirmed.json, and evidence ids against the
// evidence table. Returns a structured Error on first failure.
func ValidateClaimGraph(s store.Store, graph ClaimGraph) error {
	if graph.UpdatedAt.IsZero() {
		return NewError(CodeClaimGraphInvalid, "updated_at is required", false)
	}
	chapters, err := loadOutlineChapterIDs(s)
	if err != nil {
		return remapIOError(err)
	}
	confirmed, err := loadConfirmedKeys(s)
	if err != nil {
		return remapIOError(err)
	}
	evidenceIDs := map[string]bool{}
	if claimGraphNeedsEvidence(graph) {
		evidenceIDs, err = loadEvidenceIDs(s)
		if err != nil {
			return remapIOError(err)
		}
	}
	ids := make(map[string]bool, len(graph.Claims))
	for _, node := range graph.Claims {
		ids[node.ID] = true
	}
	seen := make(map[string]bool, len(graph.Claims))
	for i, node := range graph.Claims {
		at := fmt.Sprintf("claims[%d]", i)
		if !claimIDPattern.MatchString(node.ID) {
			return NewError(CodeClaimGraphInvalid, at+": id must match claim_NNN style, got "+quoted(node.ID), false)
		}
		if seen[node.ID] {
			return NewError(CodeClaimGraphDuplicateID, at+": duplicate claim id "+node.ID, false)
		}
		seen[node.ID] = true
		if node.Text == "" {
			return NewError(CodeClaimGraphInvalid, at+": text is required", false)
		}
		if node.ChapterID == "" {
			return NewError(CodeClaimGraphInvalid, at+": chapter_id is required", false)
		}
		if !chapters[node.ChapterID] {
			return NewError(CodeClaimGraphUnknownChapter,
				at+": chapter_id "+node.ChapterID+" does not match any outline chapter", false)
		}
		if !validSupport(node.Support) {
			return NewError(CodeClaimGraphInvalid, at+": invalid support "+quoted(node.Support), false)
		}
		if !validRiskLevel(node.RiskLevel) {
			return NewError(CodeClaimGraphInvalid, at+": invalid risk_level "+quoted(node.RiskLevel), false)
		}
		for _, key := range node.ReferenceKeys {
			if !confirmed[key] {
				return NewError(CodeClaimGraphUnconfirmedReference,
					at+": reference_key "+key+" is not in references/confirmed.json", false)
			}
		}
		for _, evidenceID := range node.EvidenceIDs {
			if !evidenceIDs[evidenceID] {
				return NewError(CodeClaimGraphUnknownEvidence,
					at+": evidence id "+evidenceID+" does not exist in the evidence table", false)
			}
		}
		for _, dup := range node.DuplicateOf {
			if !ids[dup] {
				return NewError(CodeClaimGraphInvalid,
					at+": duplicate_of references unknown claim id "+dup, false)
			}
		}
	}
	return nil
}

// BuildChapterClaimNodes projects one chapter's claims.json + citation_map.json
// into claim nodes. Reference keys are the union of the claim's own keys and
// the citation map keys of paragraphs that cite the claim. Node ids are
// assigned later by MergeChapterClaims. Fast mode marks support as skipped.
func BuildChapterClaimNodes(claims contracts.ClaimsFile, citationMap contracts.CitationMap, fastMode bool) []ClaimNode {
	mappedKeys := map[string][]string{}
	for _, mapping := range citationMap.Mappings {
		for _, claimID := range mapping.ClaimIDs {
			mappedKeys[claimID] = append(mappedKeys[claimID], mapping.ReferenceKeys...)
		}
	}
	support := ""
	if fastMode {
		support = SupportSkipped
	}
	nodes := make([]ClaimNode, 0, len(claims.Claims))
	for _, claim := range claims.Claims {
		nodes = append(nodes, ClaimNode{
			SourceClaimID: claim.ID,
			Text:          claim.Text,
			ChapterID:     claims.ChapterID,
			ReferenceKeys: unionKeys(claim.ReferenceKeys, mappedKeys[claim.ID]),
			EvidenceIDs:   append([]string{}, claim.EvidenceIDs...),
			Support:       support,
		})
	}
	return nodes
}

// MergeChapterClaims replaces the chapter's nodes inside the graph and keeps
// every other chapter untouched (chapter-level merge, never a full overwrite).
// Incoming nodes get fresh claim_NNN ids continuing after the highest id left
// in the graph, then cross-chapter duplicates are re-marked over the result.
func MergeChapterClaims(graph ClaimGraph, chapterID string, nodes []ClaimNode, updatedAt time.Time) ClaimGraph {
	merged := ClaimGraph{UpdatedAt: updatedAt}
	next := 1
	for _, node := range graph.Claims {
		if node.ChapterID == chapterID {
			continue
		}
		merged.Claims = append(merged.Claims, node)
		if n := claimNumber(node.ID); n >= next {
			next = n + 1
		}
	}
	for _, node := range nodes {
		node.ChapterID = chapterID
		node.ID = fmt.Sprintf("claim_%03d", next)
		next++
		merged.Claims = append(merged.Claims, node)
	}
	markCrossChapterDuplicates(&merged)
	return merged
}

// ExtractChapterClaimGraph loads the chapter claims and citation map, merges
// the projected nodes into the existing claim graph (a missing graph starts
// empty), and saves the result. Returns the updated graph and written paths.
func ExtractChapterClaimGraph(s store.Store, chapterID string, version int, fastMode bool, now time.Time) (ClaimGraph, []string, error) {
	claims, citationMap, err := loadChapterClaimArtifacts(s, chapterID, version)
	if err != nil {
		return ClaimGraph{}, nil, err
	}
	graph := ClaimGraph{}
	if _, statErr := os.Stat(ClaimGraphJSONPath(s)); !os.IsNotExist(statErr) {
		// Only a missing graph starts empty; any other stat or read failure
		// must surface instead of silently dropping other chapters' claims.
		graph, err = LoadClaimGraph(s)
		if err != nil {
			return ClaimGraph{}, nil, err
		}
	}
	graph = MergeChapterClaims(graph, chapterID, BuildChapterClaimNodes(claims, citationMap, fastMode), now)
	outputs, err := SaveClaimGraph(s, graph)
	if err != nil {
		return ClaimGraph{}, nil, err
	}
	return graph, outputs, nil
}

// FormatClaimGraphMarkdown renders the graph in the project Markdown style.
func FormatClaimGraphMarkdown(graph ClaimGraph) string {
	var b strings.Builder
	b.WriteString("# Claim Graph\n\n")
	fmt.Fprintf(&b, "- Updated at: %s\n\n", graph.UpdatedAt.UTC().Format(time.RFC3339))
	if len(graph.Claims) == 0 {
		b.WriteString("No claims.\n")
		return b.String()
	}
	for _, node := range graph.Claims {
		fmt.Fprintf(&b, "## %s\n\n", node.ID)
		fmt.Fprintf(&b, "- Chapter: %s\n", node.ChapterID)
		if node.SourceClaimID != "" {
			fmt.Fprintf(&b, "- Source claim: %s\n", node.SourceClaimID)
		}
		fmt.Fprintf(&b, "- Text: %s\n", node.Text)
		if len(node.ReferenceKeys) > 0 {
			fmt.Fprintf(&b, "- References: %s\n", strings.Join(node.ReferenceKeys, ", "))
		}
		if len(node.EvidenceIDs) > 0 {
			fmt.Fprintf(&b, "- Evidence: %s\n", strings.Join(node.EvidenceIDs, ", "))
		}
		if node.Support != "" {
			fmt.Fprintf(&b, "- Support: %s\n", node.Support)
		}
		if node.RiskLevel != "" {
			fmt.Fprintf(&b, "- Risk level: %s\n", node.RiskLevel)
		}
		if len(node.DuplicateOf) > 0 {
			fmt.Fprintf(&b, "- Duplicate of: %s\n", strings.Join(node.DuplicateOf, ", "))
		}
		if node.VerifierNote != "" {
			fmt.Fprintf(&b, "- Verifier note: %s\n", node.VerifierNote)
		}
		if node.NeedsRewrite {
			b.WriteString("- Needs rewrite: yes\n")
		}
		if node.NeedsHumanReview {
			b.WriteString("- Needs human review: yes\n")
		}
		b.WriteString("\n")
	}
	return b.String()
}

// markCrossChapterDuplicates re-flags near-duplicate claims across chapters.
// Earlier nodes win: a later node points at every earlier similar node in a
// different chapter. Duplicates are graded risk only, never a hard block.
func markCrossChapterDuplicates(graph *ClaimGraph) {
	tokens := make([]map[string]bool, len(graph.Claims))
	for i, node := range graph.Claims {
		tokens[i] = normalizeClaimTokens(node.Text)
		graph.Claims[i].DuplicateOf = nil
	}
	for i := range graph.Claims {
		var dups []string
		for j := 0; j < i; j++ {
			if graph.Claims[j].ChapterID == graph.Claims[i].ChapterID {
				continue
			}
			if tokenJaccard(tokens[i], tokens[j]) >= duplicateSimilarityThreshold {
				dups = append(dups, graph.Claims[j].ID)
			}
		}
		if len(dups) == 0 {
			continue
		}
		sort.Strings(dups)
		graph.Claims[i].DuplicateOf = dups
		if graph.Claims[i].RiskLevel == "" {
			graph.Claims[i].RiskLevel = RiskLow
		}
	}
}

// normalizeClaimTokens lowercases the text and keeps letter/digit runs only,
// so wording-level noise (punctuation, casing, spacing) does not hide a
// duplicate claim. ASCII words form one token per run; every other letter
// rune (e.g. CJK) becomes its own token so character-level overlap still
// detects duplicate claims in non-spaced scripts.
func normalizeClaimTokens(text string) map[string]bool {
	set := map[string]bool{}
	var run strings.Builder
	flush := func() {
		if run.Len() > 0 {
			set[run.String()] = true
			run.Reset()
		}
	}
	for _, r := range strings.ToLower(text) {
		switch {
		case ('a' <= r && r <= 'z') || ('0' <= r && r <= '9'):
			run.WriteRune(r)
		case r >= 0x80 && unicode.IsLetter(r):
			flush()
			set[string(r)] = true
		default:
			flush()
		}
	}
	flush()
	return set
}

func tokenJaccard(a, b map[string]bool) float64 {
	if len(a) == 0 || len(b) == 0 {
		return 0
	}
	intersection := 0
	for token := range a {
		if b[token] {
			intersection++
		}
	}
	union := len(a) + len(b) - intersection
	return float64(intersection) / float64(union)
}

func unionKeys(primary, extra []string) []string {
	keys := make([]string, 0, len(primary)+len(extra))
	seen := make(map[string]bool, len(primary)+len(extra))
	for _, key := range primary {
		if key != "" && !seen[key] {
			seen[key] = true
			keys = append(keys, key)
		}
	}
	for _, key := range extra {
		if key != "" && !seen[key] {
			seen[key] = true
			keys = append(keys, key)
		}
	}
	return keys
}

func claimNumber(id string) int {
	n, err := strconv.Atoi(strings.TrimPrefix(id, "claim_"))
	if err != nil {
		return 0
	}
	return n
}

func claimGraphNeedsEvidence(graph ClaimGraph) bool {
	for _, node := range graph.Claims {
		if len(node.EvidenceIDs) > 0 {
			return true
		}
	}
	return false
}

// loadChapterClaimArtifacts reads the chapter claims.json and citation_map.json
// (paths resolved by internal/artifacts) with strict JSON parsing.
func loadChapterClaimArtifacts(s store.Store, chapterID string, version int) (contracts.ClaimsFile, contracts.CitationMap, error) {
	claimsRel, err := artifacts.ClaimsPath(chapterID, version)
	if err != nil {
		return contracts.ClaimsFile{}, contracts.CitationMap{},
			NewError(CodeClaimGraphInvalid, "resolve claims path: "+err.Error(), false)
	}
	var claims contracts.ClaimsFile
	if err := store.ReadJSON(s.Path(filepath.FromSlash(claimsRel)), &claims); err != nil {
		return contracts.ClaimsFile{}, contracts.CitationMap{},
			NewError(CodeClaimGraphIOFailed, fmt.Sprintf("read %s: %v", claimsRel, err), false)
	}
	citationRel, err := artifacts.CitationMapPath(chapterID, version)
	if err != nil {
		return contracts.ClaimsFile{}, contracts.CitationMap{},
			NewError(CodeClaimGraphInvalid, "resolve citation map path: "+err.Error(), false)
	}
	var citationMap contracts.CitationMap
	if err := store.ReadJSON(s.Path(filepath.FromSlash(citationRel)), &citationMap); err != nil {
		return contracts.ClaimsFile{}, contracts.CitationMap{},
			NewError(CodeClaimGraphIOFailed, fmt.Sprintf("read %s: %v", citationRel, err), false)
	}
	if claims.ChapterID != chapterID || citationMap.ChapterID != chapterID {
		return contracts.ClaimsFile{}, contracts.CitationMap{},
			NewError(CodeClaimGraphInvalid, "chapter artifacts do not match chapter "+chapterID, false)
	}
	return claims, citationMap, nil
}

// remapIOError keeps validation-specific codes from the shared loaders but
// reports their io failures under the claim graph error code.
func remapIOError(err error) error {
	qErr, ok := AsError(err)
	if !ok {
		return NewError(CodeClaimGraphIOFailed, err.Error(), false)
	}
	switch qErr.Code {
	case CodeEvidenceIOFailed, CodeSectionPlanIOFailed:
		return NewError(CodeClaimGraphIOFailed, qErr.Message, qErr.Retryable)
	}
	return err
}

func validSupport(support string) bool {
	switch support {
	case "", SupportSupported, SupportPartiallySupported, SupportUnsupported, SupportOverstated, SupportSkipped:
		return true
	}
	return false
}

func validRiskLevel(risk string) bool {
	switch risk {
	case "", RiskHigh, RiskMedium, RiskLow:
		return true
	}
	return false
}
