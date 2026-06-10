package quality

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/store"
)

func writeOutline(t *testing.T, s store.Store, chapterIDs ...string) {
	t.Helper()
	type chapter struct {
		ChapterID string `json:"chapter_id"`
		Title     string `json:"title"`
		Goal      string `json:"goal"`
	}
	chapters := make([]chapter, 0, len(chapterIDs))
	for _, id := range chapterIDs {
		chapters = append(chapters, chapter{ChapterID: id, Title: "Title " + id, Goal: "Goal " + id})
	}
	outline := map[string]any{
		"title_suggestion": "A Survey",
		"chapters":         chapters,
	}
	if _, err := store.WriteJSON(s.Path("outline", "outline.json"), outline, store.Overwrite); err != nil {
		t.Fatalf("write outline.json: %v", err)
	}
}

func writeEvidenceTable(t *testing.T, s store.Store) {
	t.Helper()
	writeConfirmed(t, s, "doe2024deep", "smith2023survey")
	writeExtracted(t, s, "material_001", "extracted full text\n")
	if _, err := SaveEvidenceTable(s, sampleTable()); err != nil {
		t.Fatalf("SaveEvidenceTable() error = %v", err)
	}
}

func samplePlan() SectionQualityPlan {
	return SectionQualityPlan{
		GeneratedAt: time.Date(2026, 6, 10, 12, 0, 0, 0, time.UTC),
		Sections: []SectionPlan{
			{
				ChapterID:                "ch01",
				Questions:                []string{"What is the state of the art?"},
				RequiredEvidenceIDs:      []string{"ev_001"},
				RecommendedReferenceKeys: []string{"doe2024deep"},
				Boundaries:               []string{"methods belong to ch02"},
				ForbiddenGeneralizations: []string{"no universal superiority claims"},
				Gaps:                     []string{"few longitudinal studies"},
				HumanReviewHints:         []string{"check scope wording"},
			},
			{
				ChapterID:           "ch02",
				Questions:           []string{"Which methods dominate?"},
				RequiredEvidenceIDs: []string{"ev_001", "ev_002"},
			},
		},
	}
}

func TestSaveAndLoadSectionQualityPlanRoundTrip(t *testing.T) {
	s := newTestStore(t)
	writeEvidenceTable(t, s)
	writeOutline(t, s, "ch01", "ch02")

	plan := samplePlan()
	outputs, err := SaveSectionQualityPlan(s, plan)
	if err != nil {
		t.Fatalf("SaveSectionQualityPlan() error = %v", err)
	}
	if len(outputs) != 2 || outputs[0] != SectionPlanJSONRel || outputs[1] != SectionPlanMarkdownRel {
		t.Fatalf("outputs = %#v", outputs)
	}
	for _, rel := range outputs {
		if _, err := os.Stat(s.Path(filepath.FromSlash(rel))); err != nil {
			t.Fatalf("stat %s: %v", rel, err)
		}
	}

	loaded, err := LoadSectionQualityPlan(s)
	if err != nil {
		t.Fatalf("LoadSectionQualityPlan() error = %v", err)
	}
	if !loaded.GeneratedAt.Equal(plan.GeneratedAt) {
		t.Fatalf("generated_at = %v, want %v", loaded.GeneratedAt, plan.GeneratedAt)
	}
	if len(loaded.Sections) != 2 || loaded.Sections[1].RequiredEvidenceIDs[1] != "ev_002" {
		t.Fatalf("loaded sections = %#v", loaded.Sections)
	}
}

func TestSaveSectionQualityPlanRejectsUnknownEvidence(t *testing.T) {
	s := newTestStore(t)
	writeEvidenceTable(t, s)
	writeOutline(t, s, "ch01", "ch02")

	plan := samplePlan()
	plan.Sections[0].RequiredEvidenceIDs = []string{"ev_999"}

	_, err := SaveSectionQualityPlan(s, plan)
	qErr, ok := AsError(err)
	if !ok || qErr.Code != CodeSectionPlanUnknownEvidence {
		t.Fatalf("error = %v, want code %s", err, CodeSectionPlanUnknownEvidence)
	}
	if _, statErr := os.Stat(SectionQualityPlanJSONPath(s)); !os.IsNotExist(statErr) {
		t.Fatalf("section-quality-plan.json should not be written on validation failure")
	}
}

func TestSaveSectionQualityPlanRejectsMissingEvidenceTable(t *testing.T) {
	s := newTestStore(t)
	writeOutline(t, s, "ch01", "ch02")

	_, err := SaveSectionQualityPlan(s, samplePlan())
	qErr, ok := AsError(err)
	if !ok || qErr.Code != CodeSectionPlanUnknownEvidence {
		t.Fatalf("error = %v, want code %s", err, CodeSectionPlanUnknownEvidence)
	}
}

func TestSaveSectionQualityPlanRejectsUnknownChapter(t *testing.T) {
	s := newTestStore(t)
	writeEvidenceTable(t, s)
	writeOutline(t, s, "ch01")

	_, err := SaveSectionQualityPlan(s, samplePlan())
	qErr, ok := AsError(err)
	if !ok || qErr.Code != CodeSectionPlanUnknownChapter {
		t.Fatalf("error = %v, want code %s", err, CodeSectionPlanUnknownChapter)
	}
}

func TestSaveSectionQualityPlanRejectsMissingOutline(t *testing.T) {
	s := newTestStore(t)
	writeEvidenceTable(t, s)

	_, err := SaveSectionQualityPlan(s, samplePlan())
	qErr, ok := AsError(err)
	if !ok || qErr.Code != CodeSectionPlanUnknownChapter {
		t.Fatalf("error = %v, want code %s", err, CodeSectionPlanUnknownChapter)
	}
}

func TestValidateSectionQualityPlanSchemaRules(t *testing.T) {
	s := newTestStore(t)
	writeEvidenceTable(t, s)
	writeOutline(t, s, "ch01", "ch02")

	cases := []struct {
		name     string
		mutate   func(*SectionQualityPlan)
		wantCode string
	}{
		{"zero generated_at", func(p *SectionQualityPlan) { p.GeneratedAt = time.Time{} }, CodeSectionPlanInvalid},
		{"empty chapter_id", func(p *SectionQualityPlan) { p.Sections[0].ChapterID = "" }, CodeSectionPlanInvalid},
		{"duplicate chapter_id", func(p *SectionQualityPlan) { p.Sections[1].ChapterID = "ch01" }, CodeSectionPlanDuplicateChapter},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			plan := samplePlan()
			tc.mutate(&plan)
			err := ValidateSectionQualityPlan(s, plan)
			qErr, ok := AsError(err)
			if !ok || qErr.Code != tc.wantCode {
				t.Fatalf("error = %v, want code %s", err, tc.wantCode)
			}
		})
	}
}

func TestValidateSectionQualityPlanCorruptOutline(t *testing.T) {
	s := newTestStore(t)
	writeEvidenceTable(t, s)
	if _, err := store.WriteFile(s.Path("outline", "outline.json"), []byte("{not json"), store.Overwrite); err != nil {
		t.Fatalf("write corrupt outline: %v", err)
	}

	err := ValidateSectionQualityPlan(s, samplePlan())
	qErr, ok := AsError(err)
	if !ok || qErr.Code != CodeSectionPlanIOFailed {
		t.Fatalf("error = %v, want code %s", err, CodeSectionPlanIOFailed)
	}
}

func TestValidateSectionQualityPlanEmptyPlanSkipsEvidenceTable(t *testing.T) {
	s := newTestStore(t)
	plan := SectionQualityPlan{GeneratedAt: time.Date(2026, 6, 10, 12, 0, 0, 0, time.UTC)}
	if err := ValidateSectionQualityPlan(s, plan); err != nil {
		t.Fatalf("ValidateSectionQualityPlan() error = %v", err)
	}
}

func TestLoadSectionQualityPlanStrictAndMissing(t *testing.T) {
	s := newTestStore(t)

	_, err := LoadSectionQualityPlan(s)
	qErr, ok := AsError(err)
	if !ok || qErr.Code != CodeSectionPlanIOFailed {
		t.Fatalf("missing file error = %v, want code %s", err, CodeSectionPlanIOFailed)
	}

	raw := []byte("{\n  \"generated_at\": \"2026-06-10T12:00:00Z\",\n  \"sections\": [],\n  \"unknown_field\": true\n}\n")
	if _, err := store.WriteFile(SectionQualityPlanJSONPath(s), raw, store.Overwrite); err != nil {
		t.Fatalf("write raw json: %v", err)
	}
	_, err = LoadSectionQualityPlan(s)
	qErr, ok = AsError(err)
	if !ok || qErr.Code != CodeSectionPlanIOFailed || !strings.Contains(qErr.Message, "unknown_field") {
		t.Fatalf("strict read error = %v, want unknown field rejection", err)
	}
}

func TestFormatSectionQualityPlanMarkdownSnapshot(t *testing.T) {
	got := FormatSectionQualityPlanMarkdown(samplePlan())
	want := strings.Join([]string{
		"# Section Quality Plan",
		"",
		"- Generated at: 2026-06-10T12:00:00Z",
		"",
		"## ch01",
		"",
		"- Questions:",
		"  - What is the state of the art?",
		"- Required evidence:",
		"  - ev_001",
		"- Recommended references: doe2024deep",
		"- Boundaries:",
		"  - methods belong to ch02",
		"- Forbidden generalizations:",
		"  - no universal superiority claims",
		"- Gaps:",
		"  - few longitudinal studies",
		"- Human review hints:",
		"  - check scope wording",
		"",
		"## ch02",
		"",
		"- Questions:",
		"  - Which methods dominate?",
		"- Required evidence:",
		"  - ev_001",
		"  - ev_002",
		"",
	}, "\n") + "\n"
	if got != want {
		t.Fatalf("markdown snapshot mismatch\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

func TestFormatSectionQualityPlanMarkdownEmpty(t *testing.T) {
	got := FormatSectionQualityPlanMarkdown(SectionQualityPlan{GeneratedAt: time.Date(2026, 6, 10, 12, 0, 0, 0, time.UTC)})
	if !strings.Contains(got, "No section plans.") {
		t.Fatalf("empty markdown = %q", got)
	}
}

func TestSaveSectionQualityPlanTool(t *testing.T) {
	s := newTestStore(t)
	writeEvidenceTable(t, s)
	writeOutline(t, s, "ch01", "ch02")

	tool := NewSaveSectionQualityPlanTool(s)
	if tool.Name() != "save_section_quality_plan" {
		t.Fatalf("tool name = %s", tool.Name())
	}

	okArgs := `{"plan":{"generated_at":"2026-06-10T12:00:00Z","sections":[{"chapter_id":"ch01","questions":["q"],"required_evidence_ids":["ev_001"]}]}}`
	resp := execTool(t, tool, okArgs)
	if !resp.OK {
		t.Fatalf("save tool response = %#v", resp)
	}

	badArgs := `{"plan":{"generated_at":"2026-06-10T12:00:00Z","sections":[{"chapter_id":"ch01","questions":[],"required_evidence_ids":["ev_404"]}]}}`
	resp = execTool(t, tool, badArgs)
	if resp.OK || resp.Error == nil || resp.Error.Code != CodeSectionPlanUnknownEvidence {
		t.Fatalf("unknown evidence response = %#v", resp)
	}

	resp = execTool(t, tool, `{"not_plan":1}`)
	if resp.OK || resp.Error == nil || resp.Error.Code != CodeSectionPlanInvalid {
		t.Fatalf("invalid args response = %#v", resp)
	}
}

func TestLoadSectionQualityPlanTool(t *testing.T) {
	s := newTestStore(t)
	tool := NewLoadSectionQualityPlanTool(s)

	resp := execTool(t, tool, `{}`)
	if resp.OK || resp.Error == nil || resp.Error.Code != CodeSectionPlanIOFailed {
		t.Fatalf("missing plan response = %#v", resp)
	}

	writeEvidenceTable(t, s)
	writeOutline(t, s, "ch01", "ch02")
	if _, err := SaveSectionQualityPlan(s, samplePlan()); err != nil {
		t.Fatalf("SaveSectionQualityPlan() error = %v", err)
	}

	resp = execTool(t, tool, `{}`)
	if !resp.OK {
		t.Fatalf("load tool response = %#v", resp)
	}
	data, err := json.Marshal(resp.Data)
	if err != nil {
		t.Fatalf("marshal data: %v", err)
	}
	var loaded SectionQualityPlan
	if err := json.Unmarshal(data, &loaded); err != nil {
		t.Fatalf("unmarshal plan: %v", err)
	}
	if len(loaded.Sections) != 2 || loaded.Sections[0].ChapterID != "ch01" {
		t.Fatalf("loaded via tool = %#v", loaded)
	}
}
