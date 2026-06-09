package references

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/contracts"
	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/store"
)

func TestWriteCandidates_CreatesJSONAndMarkdown(t *testing.T) {
	tmpDir := t.TempDir()
	s := store.New(tmpDir)
	if err := store.EnsureLayout(s); err != nil {
		t.Fatal(err)
	}

	candidates := []contracts.ReferenceCandidate{
		{
			ID:      "cand_001",
			Title:   "Test Paper",
			Authors: []string{"Author A", "Author B"},
			Year:    2023,
			Source:  "test",
			DOI:     "10.1234/test",
			Status:  "pending",
		},
	}

	outputs, err := WriteCandidates(s, candidates)
	if err != nil {
		t.Fatalf("WriteCandidates failed: %v", err)
	}

	if len(outputs) != 2 {
		t.Fatalf("expected 2 outputs, got %d", len(outputs))
	}

	// Check JSON file
	jsonPath := s.Path("references", "candidates.json")
	jsonData, err := os.ReadFile(jsonPath)
	if err != nil {
		t.Fatalf("failed to read JSON: %v", err)
	}

	var candidatesData contracts.ReferenceCandidates
	if err := json.Unmarshal(jsonData, &candidatesData); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}

	if len(candidatesData.Items) != 1 {
		t.Errorf("expected 1 candidate in JSON, got %d", len(candidatesData.Items))
	}
	if candidatesData.Items[0].ID != "cand_001" {
		t.Errorf("expected ID cand_001, got %s", candidatesData.Items[0].ID)
	}

	// Check markdown file
	mdPath := s.Path("references", "candidates.md")
	mdData, err := os.ReadFile(mdPath)
	if err != nil {
		t.Fatalf("failed to read markdown: %v", err)
	}

	mdContent := string(mdData)
	if !strings.Contains(mdContent, "# Reference Candidates") {
		t.Error("markdown should contain header")
	}
	if !strings.Contains(mdContent, "## cand_001") {
		t.Error("markdown should contain candidate ID")
	}
	if !strings.Contains(mdContent, "Test Paper") {
		t.Error("markdown should contain title")
	}
	if !strings.Contains(mdContent, "Author A, Author B") {
		t.Error("markdown should contain authors")
	}
}

func TestWriteCandidates_EmptyList(t *testing.T) {
	tmpDir := t.TempDir()
	s := store.New(tmpDir)
	if err := store.EnsureLayout(s); err != nil {
		t.Fatal(err)
	}

	candidates := []contracts.ReferenceCandidate{}

	outputs, err := WriteCandidates(s, candidates)
	if err != nil {
		t.Fatalf("WriteCandidates failed: %v", err)
	}

	if len(outputs) != 2 {
		t.Fatalf("expected 2 outputs, got %d", len(outputs))
	}

	mdPath := s.Path("references", "candidates.md")
	mdData, err := os.ReadFile(mdPath)
	if err != nil {
		t.Fatalf("failed to read markdown: %v", err)
	}

	mdContent := string(mdData)
	if !strings.Contains(mdContent, "No reference candidates") {
		t.Error("markdown should indicate no candidates")
	}
}

func TestFormatCandidatesMarkdown_AllFields(t *testing.T) {
	candidates := []contracts.ReferenceCandidate{
		{
			ID:             "cand_001",
			Title:          "Complete Paper",
			Authors:        []string{"Author A"},
			Year:           2023,
			Source:         "test",
			DOI:            "10.1234/test",
			URL:            "https://example.com",
			Abstract:       "This is an abstract.",
			Venue:          "Conference 2023",
			CitationCount:  42,
			DedupeGroup:    "doi:10.1234/test",
			Status:         "pending",
		},
	}

	md := FormatCandidatesMarkdown(candidates)

	expectations := []string{
		"# Reference Candidates",
		"## cand_001",
		"- Title: Complete Paper",
		"- Authors: Author A",
		"- Year: 2023",
		"- Source: test",
		"- DOI: 10.1234/test",
		"- URL: https://example.com",
		"- Venue: Conference 2023",
		"- Citation count: 42",
		"- Dedupe group: doi:10.1234/test",
		"This is an abstract.",
	}

	for _, expected := range expectations {
		if !strings.Contains(md, expected) {
			t.Errorf("markdown should contain %q", expected)
		}
	}
}

func TestFormatCandidatesMarkdown_MinimalFields(t *testing.T) {
	candidates := []contracts.ReferenceCandidate{
		{
			ID:      "cand_001",
			Title:   "Minimal Paper",
			Authors: []string{},
			Source:  "test",
			Status:  "pending",
		},
	}

	md := FormatCandidatesMarkdown(candidates)

	if !strings.Contains(md, "## cand_001") {
		t.Error("markdown should contain candidate ID")
	}
	if !strings.Contains(md, "- Title: Minimal Paper") {
		t.Error("markdown should contain title")
	}
	if strings.Contains(md, "Year:") {
		t.Error("markdown should not contain year when zero")
	}
	if strings.Contains(md, "DOI:") {
		t.Error("markdown should not contain DOI when empty")
	}
}

func TestFormatCandidatesMarkdown_MultipleCandidates(t *testing.T) {
	candidates := []contracts.ReferenceCandidate{
		{ID: "cand_001", Title: "Paper 1", Authors: []string{"A"}, Source: "test", Status: "pending"},
		{ID: "cand_002", Title: "Paper 2", Authors: []string{"B"}, Source: "test", Status: "pending"},
		{ID: "cand_003", Title: "Paper 3", Authors: []string{"C"}, Source: "test", Status: "pending"},
	}

	md := FormatCandidatesMarkdown(candidates)

	for _, id := range []string{"cand_001", "cand_002", "cand_003"} {
		if !strings.Contains(md, "## "+id) {
			t.Errorf("markdown should contain %s", id)
		}
	}
}
