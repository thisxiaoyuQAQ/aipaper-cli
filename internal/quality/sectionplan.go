package quality

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/store"
)

// Structured error codes for section quality plan validation.
const (
	CodeSectionPlanInvalid          = "section_plan_invalid"
	CodeSectionPlanDuplicateChapter = "section_plan_duplicate_chapter"
	CodeSectionPlanUnknownChapter   = "section_plan_unknown_chapter"
	CodeSectionPlanUnknownEvidence  = "section_plan_unknown_evidence"
	CodeSectionPlanIOFailed         = "section_plan_io_failed"
)

// SectionQualityPlan is the pre-writing per-chapter quality plan persisted at
// quality/section-quality-plan.json. See docs/interfaces/quality.md.
type SectionQualityPlan struct {
	GeneratedAt time.Time     `json:"generated_at"`
	Sections    []SectionPlan `json:"sections"`
}

// SectionPlan is the quality plan of one outline chapter.
type SectionPlan struct {
	ChapterID                string   `json:"chapter_id"` // must match an outline chapter
	Questions                []string `json:"questions"`
	RequiredEvidenceIDs      []string `json:"required_evidence_ids"` // must exist in the evidence table
	RecommendedReferenceKeys []string `json:"recommended_reference_keys,omitempty"`
	Boundaries               []string `json:"boundaries,omitempty"`
	ForbiddenGeneralizations []string `json:"forbidden_generalizations,omitempty"`
	Gaps                     []string `json:"gaps,omitempty"`
	HumanReviewHints         []string `json:"human_review_hints,omitempty"`
}

// SectionPlanJSONRel / SectionPlanMarkdownRel are paths relative to the store root.
const (
	SectionPlanJSONRel     = "quality/section-quality-plan.json"
	SectionPlanMarkdownRel = "quality/section-quality-plan.md"
)

// SectionQualityPlanJSONPath returns the absolute JSON path inside the store.
func SectionQualityPlanJSONPath(s store.Store) string {
	return s.Path(filepath.FromSlash(SectionPlanJSONRel))
}

// SectionQualityPlanMarkdownPath returns the absolute Markdown path inside the store.
func SectionQualityPlanMarkdownPath(s store.Store) string {
	return s.Path(filepath.FromSlash(SectionPlanMarkdownRel))
}

// SaveSectionQualityPlan validates the plan against the outline and the
// evidence table, then atomically writes JSON + Markdown.
// Returns the written paths relative to the store root (forward slashes).
func SaveSectionQualityPlan(s store.Store, plan SectionQualityPlan) ([]string, error) {
	if err := ValidateSectionQualityPlan(s, plan); err != nil {
		return nil, err
	}
	if _, err := store.WriteJSON(SectionQualityPlanJSONPath(s), plan, store.Overwrite); err != nil {
		return nil, NewError(CodeSectionPlanIOFailed, fmt.Sprintf("write %s: %v", SectionPlanJSONRel, err), true)
	}
	md := FormatSectionQualityPlanMarkdown(plan)
	if _, err := store.WriteFile(SectionQualityPlanMarkdownPath(s), []byte(md), store.Overwrite); err != nil {
		return nil, NewError(CodeSectionPlanIOFailed, fmt.Sprintf("write %s: %v", SectionPlanMarkdownRel, err), true)
	}
	return []string{SectionPlanJSONRel, SectionPlanMarkdownRel}, nil
}

// LoadSectionQualityPlan reads quality/section-quality-plan.json with strict
// JSON parsing. A missing file returns a structured error.
func LoadSectionQualityPlan(s store.Store) (SectionQualityPlan, error) {
	var plan SectionQualityPlan
	err := store.ReadJSON(SectionQualityPlanJSONPath(s), &plan)
	if errors.Is(err, os.ErrNotExist) {
		return SectionQualityPlan{}, NewError(CodeSectionPlanIOFailed, "section quality plan not found: "+SectionPlanJSONRel, false)
	}
	if err != nil {
		return SectionQualityPlan{}, NewError(CodeSectionPlanIOFailed, fmt.Sprintf("read %s: %v", SectionPlanJSONRel, err), false)
	}
	return plan, nil
}

// ValidateSectionQualityPlan checks that every chapter_id matches an outline
// chapter and every required evidence id exists in the evidence table.
// Returns a structured Error on first failure.
func ValidateSectionQualityPlan(s store.Store, plan SectionQualityPlan) error {
	if plan.GeneratedAt.IsZero() {
		return NewError(CodeSectionPlanInvalid, "generated_at is required", false)
	}
	chapters, err := loadOutlineChapterIDs(s)
	if err != nil {
		return err
	}
	seen := make(map[string]bool, len(plan.Sections))
	needsEvidence := false
	for _, section := range plan.Sections {
		if len(section.RequiredEvidenceIDs) > 0 {
			needsEvidence = true
			break
		}
	}
	evidenceIDs := map[string]bool{}
	if needsEvidence {
		evidenceIDs, err = loadEvidenceIDs(s)
		if err != nil {
			return err
		}
	}
	for i, section := range plan.Sections {
		at := fmt.Sprintf("sections[%d]", i)
		if section.ChapterID == "" {
			return NewError(CodeSectionPlanInvalid, at+": chapter_id is required", false)
		}
		if seen[section.ChapterID] {
			return NewError(CodeSectionPlanDuplicateChapter, at+": duplicate chapter_id "+section.ChapterID, false)
		}
		seen[section.ChapterID] = true
		if !chapters[section.ChapterID] {
			return NewError(CodeSectionPlanUnknownChapter,
				at+": chapter_id "+section.ChapterID+" does not match any outline chapter", false)
		}
		for _, evidenceID := range section.RequiredEvidenceIDs {
			if !evidenceIDs[evidenceID] {
				return NewError(CodeSectionPlanUnknownEvidence,
					at+": required evidence id "+evidenceID+" does not exist in the evidence table", false)
			}
		}
	}
	return nil
}

// FormatSectionQualityPlanMarkdown renders the plan in the project Markdown style.
func FormatSectionQualityPlanMarkdown(plan SectionQualityPlan) string {
	var b strings.Builder
	b.WriteString("# Section Quality Plan\n\n")
	fmt.Fprintf(&b, "- Generated at: %s\n\n", plan.GeneratedAt.UTC().Format(time.RFC3339))
	if len(plan.Sections) == 0 {
		b.WriteString("No section plans.\n")
		return b.String()
	}
	for _, section := range plan.Sections {
		fmt.Fprintf(&b, "## %s\n\n", section.ChapterID)
		writeMarkdownList(&b, "Questions", section.Questions)
		writeMarkdownList(&b, "Required evidence", section.RequiredEvidenceIDs)
		if len(section.RecommendedReferenceKeys) > 0 {
			fmt.Fprintf(&b, "- Recommended references: %s\n", strings.Join(section.RecommendedReferenceKeys, ", "))
		}
		writeMarkdownList(&b, "Boundaries", section.Boundaries)
		writeMarkdownList(&b, "Forbidden generalizations", section.ForbiddenGeneralizations)
		writeMarkdownList(&b, "Gaps", section.Gaps)
		writeMarkdownList(&b, "Human review hints", section.HumanReviewHints)
		b.WriteString("\n")
	}
	return b.String()
}

// loadOutlineChapterIDs reads chapter ids from outline/outline.json with a
// tolerant parse (the outline carries many Architect fields beyond ids).
// A missing outline means zero known chapters, mirroring confirmed.json.
func loadOutlineChapterIDs(s store.Store) (map[string]bool, error) {
	data, err := os.ReadFile(s.Path("outline", "outline.json"))
	if errors.Is(err, os.ErrNotExist) {
		return map[string]bool{}, nil
	}
	if err != nil {
		return nil, NewError(CodeSectionPlanIOFailed, fmt.Sprintf("read outline/outline.json: %v", err), false)
	}
	var outline struct {
		Chapters []struct {
			ChapterID string `json:"chapter_id"`
		} `json:"chapters"`
	}
	if err := json.Unmarshal(data, &outline); err != nil {
		return nil, NewError(CodeSectionPlanIOFailed, fmt.Sprintf("parse outline/outline.json: %v", err), false)
	}
	ids := make(map[string]bool, len(outline.Chapters))
	for _, chapter := range outline.Chapters {
		ids[chapter.ChapterID] = true
	}
	return ids, nil
}

// loadEvidenceIDs reads the evidence table ids. A missing table means zero
// known evidence ids; other read failures are reported as io errors.
func loadEvidenceIDs(s store.Store) (map[string]bool, error) {
	table, err := LoadEvidenceTable(s)
	if err != nil {
		if _, statErr := os.Stat(EvidenceTableJSONPath(s)); errors.Is(statErr, os.ErrNotExist) {
			return map[string]bool{}, nil
		}
		return nil, NewError(CodeSectionPlanIOFailed, "read evidence table: "+err.Error(), false)
	}
	ids := make(map[string]bool, len(table.Items))
	for _, item := range table.Items {
		ids[item.ID] = true
	}
	return ids, nil
}
