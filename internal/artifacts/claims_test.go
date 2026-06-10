package artifacts

import (
	"path/filepath"
	"testing"

	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/contracts"
	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/store"
)

// Legacy claims.json (pre quality engine, no evidence_ids) must still load
// under strict JSON reading and only surface compatibility warnings.
func TestClaimsFileReadsLegacyFormatWithoutEvidenceIDs(t *testing.T) {
	s := store.NewAt(t.TempDir())
	rel, err := ClaimsPath("ch01", 1)
	if err != nil {
		t.Fatalf("ClaimsPath() error = %v", err)
	}
	legacy := `{
  "chapter_id": "ch01",
  "draft_version": 1,
  "claims": [
    {
      "id": "ch01_claim_001",
      "text": "Old runs carry no evidence bindings.",
      "importance": "high",
      "reference_keys": ["smith2024Rag"],
      "confidence": 0.8
    }
  ]
}`
	if _, err := store.WriteFile(s.Path(filepath.FromSlash(rel)), []byte(legacy), store.Overwrite); err != nil {
		t.Fatalf("WriteFile(legacy claims) error = %v", err)
	}

	var claims contracts.ClaimsFile
	if err := store.ReadJSON(s.Path(filepath.FromSlash(rel)), &claims); err != nil {
		t.Fatalf("ReadJSON(legacy claims) error = %v", err)
	}
	if len(claims.Claims) != 1 || claims.Claims[0].EvidenceIDs != nil {
		t.Fatalf("legacy claims = %#v", claims.Claims)
	}

	warnings := ClaimEvidenceWarnings(claims)
	if len(warnings) != 1 {
		t.Fatalf("warnings = %#v", warnings)
	}
	if warnings[0].Code != CodeClaimMissingEvidence || warnings[0].ID != "ch01_claim_001" {
		t.Fatalf("warning = %#v", warnings[0])
	}
}

func TestClaimsFileRoundTripsEvidenceIDs(t *testing.T) {
	s := store.NewAt(t.TempDir())
	rel, err := ClaimsPath("ch01", 1)
	if err != nil {
		t.Fatalf("ClaimsPath() error = %v", err)
	}
	claims := contracts.ClaimsFile{
		ChapterID:    "ch01",
		DraftVersion: 1,
		Claims: []contracts.Claim{{
			ID:            "ch01_claim_001",
			Text:          "New claims bind evidence.",
			Importance:    "high",
			ReferenceKeys: []string{"smith2024Rag"},
			EvidenceIDs:   []string{"ev_001", "ev_002"},
		}},
	}
	if _, err := store.WriteJSON(s.Path(filepath.FromSlash(rel)), claims, store.Overwrite); err != nil {
		t.Fatalf("WriteJSON(claims) error = %v", err)
	}

	var loaded contracts.ClaimsFile
	if err := store.ReadJSON(s.Path(filepath.FromSlash(rel)), &loaded); err != nil {
		t.Fatalf("ReadJSON(claims) error = %v", err)
	}
	got := loaded.Claims[0].EvidenceIDs
	if len(got) != 2 || got[0] != "ev_001" || got[1] != "ev_002" {
		t.Fatalf("evidence ids = %#v", got)
	}
	if warnings := ClaimEvidenceWarnings(loaded); len(warnings) != 0 {
		t.Fatalf("unexpected warnings = %#v", warnings)
	}
}
