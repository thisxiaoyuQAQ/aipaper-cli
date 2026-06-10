// Package quality implements the Quality Engine artifacts: evidence table,
// section quality plan, claim graph, and quality gate logic (modules 23-31).
package quality

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/contracts"
	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/store"
)

// Evidence depth levels, progressive: metadata only -> abstract -> snippet -> fulltext excerpt.
const (
	DepthMetadataOnly    = "metadata_only"
	DepthAbstract        = "abstract"
	DepthSnippet         = "snippet"
	DepthFulltextExcerpt = "fulltext_excerpt"
)

// Evidence confidence levels.
const (
	ConfidenceHigh   = "high"
	ConfidenceMedium = "medium"
	ConfidenceLow    = "low"
)

// Structured error codes for evidence table validation.
const (
	CodeEvidenceInvalid              = "evidence_invalid"
	CodeEvidenceDuplicateID          = "evidence_duplicate_id"
	CodeEvidenceUnconfirmedReference = "evidence_unconfirmed_reference"
	CodeEvidenceDepthUnsupported     = "evidence_depth_unsupported"
	CodeEvidenceIOFailed             = "evidence_io_failed"
)

// Error is the structured tool/validation error shared by quality artifacts.
// Shape matches the project convention {code,message,retryable,details}.
type Error struct {
	Code      string         `json:"code"`
	Message   string         `json:"message"`
	Retryable bool           `json:"retryable"`
	Details   map[string]any `json:"details,omitempty"`
}

func (e Error) Error() string {
	if e.Code == "" {
		return e.Message
	}
	if e.Message == "" {
		return e.Code
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// NewError builds a structured quality error.
func NewError(code, message string, retryable bool) Error {
	return Error{Code: code, Message: message, Retryable: retryable}
}

// AsError extracts a structured quality Error from err when possible.
func AsError(err error) (Error, bool) {
	var qErr Error
	if errors.As(err, &qErr) {
		return qErr, true
	}
	return Error{}, false
}

// EvidenceTable is the evidence foundation persisted at quality/evidence-table.json.
type EvidenceTable struct {
	GeneratedAt time.Time  `json:"generated_at"`
	Items       []Evidence `json:"items"`
}

// Evidence is one entry of the evidence table. See docs/interfaces/quality.md.
type Evidence struct {
	ID           string   `json:"id"`                    // ev_001 style
	ReferenceKey string   `json:"reference_key"`         // must exist in references/confirmed.json
	MaterialID   string   `json:"material_id,omitempty"` // material_001 when sourced from user material
	Depth        string   `json:"depth"`                 // metadata_only|abstract|snippet|fulltext_excerpt
	Topics       []string `json:"topics"`
	KeyFindings  []string `json:"key_findings"`
	Method       string   `json:"method,omitempty"`
	Subjects     string   `json:"subjects,omitempty"`
	Limitations  []string `json:"limitations,omitempty"`
	Excerpt      string   `json:"excerpt,omitempty"` // snippet level and above only
	Confidence   string   `json:"confidence"`        // high|medium|low
	Coverage     string   `json:"coverage,omitempty"`
	RiskFlags    []string `json:"risk_flags,omitempty"`
}

var evidenceIDPattern = regexp.MustCompile(`^ev_\d{3,}$`)

// EvidenceTableJSONRel / EvidenceTableMarkdownRel are paths relative to the store root.
const (
	EvidenceTableJSONRel     = "quality/evidence-table.json"
	EvidenceTableMarkdownRel = "quality/evidence-table.md"
)

// EvidenceTableJSONPath returns the absolute JSON path inside the store.
func EvidenceTableJSONPath(s store.Store) string {
	return s.Path(filepath.FromSlash(EvidenceTableJSONRel))
}

// EvidenceTableMarkdownPath returns the absolute Markdown path inside the store.
func EvidenceTableMarkdownPath(s store.Store) string {
	return s.Path(filepath.FromSlash(EvidenceTableMarkdownRel))
}

// SaveEvidenceTable validates the table against confirmed references and
// material extracted texts, then atomically writes JSON + Markdown.
// Returns the written paths relative to the store root (forward slashes).
func SaveEvidenceTable(s store.Store, table EvidenceTable) ([]string, error) {
	if err := ValidateEvidenceTable(s, table); err != nil {
		return nil, err
	}
	if _, err := store.WriteJSON(EvidenceTableJSONPath(s), table, store.Overwrite); err != nil {
		return nil, NewError(CodeEvidenceIOFailed, fmt.Sprintf("write %s: %v", EvidenceTableJSONRel, err), true)
	}
	md := FormatEvidenceTableMarkdown(table)
	if _, err := store.WriteFile(EvidenceTableMarkdownPath(s), []byte(md), store.Overwrite); err != nil {
		return nil, NewError(CodeEvidenceIOFailed, fmt.Sprintf("write %s: %v", EvidenceTableMarkdownRel, err), true)
	}
	return []string{EvidenceTableJSONRel, EvidenceTableMarkdownRel}, nil
}

// LoadEvidenceTable reads quality/evidence-table.json with strict JSON parsing.
// A missing file returns os.ErrNotExist wrapped in a structured error.
func LoadEvidenceTable(s store.Store) (EvidenceTable, error) {
	var table EvidenceTable
	err := store.ReadJSON(EvidenceTableJSONPath(s), &table)
	if errors.Is(err, os.ErrNotExist) {
		return EvidenceTable{}, NewError(CodeEvidenceIOFailed, "evidence table not found: "+EvidenceTableJSONRel, false)
	}
	if err != nil {
		return EvidenceTable{}, NewError(CodeEvidenceIOFailed, fmt.Sprintf("read %s: %v", EvidenceTableJSONRel, err), false)
	}
	return table, nil
}

// ValidateEvidenceTable checks schema rules, confirmed reference keys, and
// progressive depth constraints. Returns a structured Error on first failure.
func ValidateEvidenceTable(s store.Store, table EvidenceTable) error {
	if table.GeneratedAt.IsZero() {
		return NewError(CodeEvidenceInvalid, "generated_at is required", false)
	}
	confirmed, err := loadConfirmedKeys(s)
	if err != nil {
		return err
	}
	seen := make(map[string]bool, len(table.Items))
	for i, ev := range table.Items {
		at := fmt.Sprintf("items[%d]", i)
		if !evidenceIDPattern.MatchString(ev.ID) {
			return NewError(CodeEvidenceInvalid, at+": id must match ev_NNN style, got "+quoted(ev.ID), false)
		}
		if seen[ev.ID] {
			return NewError(CodeEvidenceDuplicateID, at+": duplicate evidence id "+ev.ID, false)
		}
		seen[ev.ID] = true
		if ev.ReferenceKey == "" {
			return NewError(CodeEvidenceInvalid, at+": reference_key is required", false)
		}
		if !confirmed[ev.ReferenceKey] {
			return NewError(CodeEvidenceUnconfirmedReference,
				at+": reference_key "+ev.ReferenceKey+" is not in references/confirmed.json", false)
		}
		if !validDepth(ev.Depth) {
			return NewError(CodeEvidenceInvalid, at+": invalid depth "+quoted(ev.Depth), false)
		}
		if !validConfidence(ev.Confidence) {
			return NewError(CodeEvidenceInvalid, at+": invalid confidence "+quoted(ev.Confidence), false)
		}
		if err := validateDepthEvidence(s, at, ev); err != nil {
			return err
		}
	}
	return nil
}

// validateDepthEvidence enforces the progressive depth rule: snippet and
// fulltext_excerpt require an extracted material text to actually exist, and
// excerpts are only allowed at snippet level or above.
func validateDepthEvidence(s store.Store, at string, ev Evidence) error {
	snippetOrAbove := ev.Depth == DepthSnippet || ev.Depth == DepthFulltextExcerpt
	if !snippetOrAbove {
		if ev.Excerpt != "" {
			return NewError(CodeEvidenceInvalid,
				at+": excerpt is only allowed at snippet level or above, depth is "+ev.Depth, false)
		}
		return nil
	}
	if ev.MaterialID == "" {
		return NewError(CodeEvidenceDepthUnsupported,
			at+": depth "+ev.Depth+" requires material_id with extracted text", false)
	}
	extracted := s.Path("materials", "extracted", ev.MaterialID+".md")
	info, err := os.Stat(extracted)
	if err != nil || info.IsDir() || info.Size() == 0 {
		return NewError(CodeEvidenceDepthUnsupported,
			at+": depth "+ev.Depth+" requires non-empty extracted text for material "+ev.MaterialID, false)
	}
	return nil
}

// FormatEvidenceTableMarkdown renders the table in the project Markdown style.
func FormatEvidenceTableMarkdown(table EvidenceTable) string {
	var b strings.Builder
	b.WriteString("# Evidence Table\n\n")
	fmt.Fprintf(&b, "- Generated at: %s\n\n", table.GeneratedAt.UTC().Format(time.RFC3339))
	if len(table.Items) == 0 {
		b.WriteString("No evidence items.\n")
		return b.String()
	}
	for _, ev := range table.Items {
		fmt.Fprintf(&b, "## %s\n\n", ev.ID)
		fmt.Fprintf(&b, "- Reference key: %s\n", ev.ReferenceKey)
		if ev.MaterialID != "" {
			fmt.Fprintf(&b, "- Material: %s\n", ev.MaterialID)
		}
		fmt.Fprintf(&b, "- Depth: %s\n", ev.Depth)
		fmt.Fprintf(&b, "- Confidence: %s\n", ev.Confidence)
		if ev.Coverage != "" {
			fmt.Fprintf(&b, "- Coverage: %s\n", ev.Coverage)
		}
		if len(ev.Topics) > 0 {
			fmt.Fprintf(&b, "- Topics: %s\n", strings.Join(ev.Topics, ", "))
		}
		if ev.Method != "" {
			fmt.Fprintf(&b, "- Method: %s\n", ev.Method)
		}
		if ev.Subjects != "" {
			fmt.Fprintf(&b, "- Subjects: %s\n", ev.Subjects)
		}
		if len(ev.RiskFlags) > 0 {
			fmt.Fprintf(&b, "- Risk flags: %s\n", strings.Join(ev.RiskFlags, ", "))
		}
		writeMarkdownList(&b, "Key findings", ev.KeyFindings)
		writeMarkdownList(&b, "Limitations", ev.Limitations)
		if ev.Excerpt != "" {
			fmt.Fprintf(&b, "\n> %s\n", strings.ReplaceAll(ev.Excerpt, "\n", "\n> "))
		}
		b.WriteString("\n")
	}
	return b.String()
}

func writeMarkdownList(b *strings.Builder, label string, items []string) {
	if len(items) == 0 {
		return
	}
	fmt.Fprintf(b, "- %s:\n", label)
	for _, item := range items {
		fmt.Fprintf(b, "  - %s\n", item)
	}
}

func loadConfirmedKeys(s store.Store) (map[string]bool, error) {
	var confirmed contracts.ConfirmedReferences
	err := store.ReadJSON(s.Path("references", "confirmed.json"), &confirmed)
	if errors.Is(err, os.ErrNotExist) {
		return map[string]bool{}, nil
	}
	if err != nil {
		return nil, NewError(CodeEvidenceIOFailed, fmt.Sprintf("read references/confirmed.json: %v", err), false)
	}
	keys := make(map[string]bool, len(confirmed.Items))
	for _, item := range confirmed.Items {
		keys[item.Key] = true
	}
	return keys, nil
}

func validDepth(depth string) bool {
	switch depth {
	case DepthMetadataOnly, DepthAbstract, DepthSnippet, DepthFulltextExcerpt:
		return true
	}
	return false
}

func validConfidence(confidence string) bool {
	switch confidence {
	case ConfidenceHigh, ConfidenceMedium, ConfidenceLow:
		return true
	}
	return false
}

func quoted(s string) string {
	return fmt.Sprintf("%q", s)
}
