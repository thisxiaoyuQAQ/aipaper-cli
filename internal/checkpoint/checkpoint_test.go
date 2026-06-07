package checkpoint

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/contracts"
	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/store"
)

func TestRecordValidateLatestAndIdempotentRepeat(t *testing.T) {
	s := store.NewAt(t.TempDir())
	artifact := writeArtifact(t, s, "drafts/ch01/draft.md", "draft")
	cp := Checkpoint{
		Step:      7,
		Phase:     "draft_chapter",
		CreatedAt: fixedTime(),
		Outputs: []OutputArtifact{{
			Kind:   "draft",
			Path:   "drafts/ch01/draft.md",
			SHA256: artifact.SHA256,
		}},
		NextExpected: "review_chapter",
	}
	progress := contracts.Progress{
		Phase:             "draft_chapter",
		Status:            "running",
		CompletedChapters: []string{},
		PendingChapters:   []string{"ch01"},
		UpdatedAt:         fixedTime(),
	}

	if err := Record(s, cp, progress); err != nil {
		t.Fatalf("Record() error = %v", err)
	}
	if err := Record(s, cp, progress); err != nil {
		t.Fatalf("Record(idempotent repeat) error = %v", err)
	}

	validation := ValidateLatest(s)
	if !validation.OK {
		t.Fatalf("ValidateLatest() = %#v", validation)
	}
	if validation.Step != 7 || validation.Phase != "draft_chapter" || validation.Next != "review_chapter" {
		t.Fatalf("validation metadata = %#v", validation)
	}
	if len(validation.Checked) != 1 || validation.Checked[0] != "drafts/ch01/draft.md" {
		t.Fatalf("checked = %#v", validation.Checked)
	}
}

func TestRecordRejectsConflictingRepeatedStep(t *testing.T) {
	s := store.NewAt(t.TempDir())
	firstArtifact := writeArtifact(t, s, "drafts/ch01/draft.md", "draft")
	secondArtifact := writeArtifact(t, s, "reviews/ch01/review.json", `{"passed":true}`)
	progress := contracts.Progress{
		Phase:             "draft_chapter",
		Status:            "running",
		CompletedChapters: []string{},
		PendingChapters:   []string{"ch01"},
		UpdatedAt:         fixedTime(),
	}

	if err := Record(s, Checkpoint{
		Step:      7,
		Phase:     "draft_chapter",
		CreatedAt: fixedTime(),
		Outputs: []OutputArtifact{{
			Kind:   "draft",
			Path:   "drafts/ch01/draft.md",
			SHA256: firstArtifact.SHA256,
		}},
	}, progress); err != nil {
		t.Fatalf("Record(first) error = %v", err)
	}

	err := Record(s, Checkpoint{
		Step:      7,
		Phase:     "review_chapter",
		CreatedAt: fixedTime(),
		Outputs: []OutputArtifact{{
			Kind:   "review",
			Path:   "reviews/ch01/review.json",
			SHA256: secondArtifact.SHA256,
		}},
	}, progress)
	if err == nil || !strings.Contains(err.Error(), "STORE_CHECKPOINT_CONFLICT") || !strings.Contains(err.Error(), "artifact conflict") {
		t.Fatalf("Record(conflict) error = %v", err)
	}
}

func TestValidateLatestReportsMissingLatest(t *testing.T) {
	validation := ValidateLatest(store.NewAt(t.TempDir()))
	if validation.OK {
		t.Fatalf("ValidateLatest() unexpectedly OK")
	}
	if !hasError(validation.Errors, "latest checkpoint does not exist") {
		t.Fatalf("errors = %#v", validation.Errors)
	}
}

func TestValidateLatestReportsArtifactFailures(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		sha      string
		write    bool
		wantPart string
	}{
		{name: "absolute", path: "/tmp/escape.txt", sha: "x", wantPart: "relative"},
		{name: "backslash", path: `drafts\ch01\draft.md`, sha: "x", wantPart: "forward slashes"},
		{name: "escape", path: "../outside.txt", sha: "x", wantPart: "escapes store root"},
		{name: "missing", path: "drafts/ch01/missing.md", sha: "x", wantPart: "read drafts/ch01/missing.md"},
		{name: "hash mismatch", path: "drafts/ch01/draft.md", sha: "bad", write: true, wantPart: "hash mismatch"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := store.NewAt(t.TempDir())
			if tt.write {
				writeArtifact(t, s, tt.path, "draft")
			}
			writeLatest(t, s, Checkpoint{
				Step:      1,
				Phase:     "draft_chapter",
				CreatedAt: fixedTime(),
				Outputs: []OutputArtifact{{
					Kind:   "draft",
					Path:   tt.path,
					SHA256: tt.sha,
				}},
			})

			validation := ValidateLatest(s)
			if validation.OK {
				t.Fatalf("ValidateLatest() unexpectedly OK")
			}
			if !hasError(validation.Errors, tt.wantPart) {
				t.Fatalf("errors = %#v, want %q", validation.Errors, tt.wantPart)
			}
		})
	}
}

func writeArtifact(t *testing.T, s store.Store, relPath, data string) store.WriteResult {
	t.Helper()
	result, err := store.WriteFile(s.Path(filepath.FromSlash(relPath)), []byte(data), store.CreateOnly)
	if err != nil {
		t.Fatalf("WriteFile(%s) error = %v", relPath, err)
	}
	return result
}

func writeLatest(t *testing.T, s store.Store, cp Checkpoint) {
	t.Helper()
	if err := store.EnsureLayout(s); err != nil {
		t.Fatalf("EnsureLayout() error = %v", err)
	}
	if _, err := store.WriteJSON(s.LatestCheckpointPath(), cp, store.Overwrite); err != nil {
		t.Fatalf("WriteJSON(latest) error = %v", err)
	}
}

func hasError(errors []string, part string) bool {
	for _, err := range errors {
		if strings.Contains(err, part) {
			return true
		}
	}
	return false
}

func fixedTime() time.Time {
	return time.Date(2026, 6, 7, 12, 0, 0, 0, time.UTC)
}
