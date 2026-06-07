package app

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/checkpoint"
	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/config"
	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/contracts"
	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/store"
)

func TestRecoverBuildsPromptMarksRunAndKeepsArtifacts(t *testing.T) {
	workDir := t.TempDir()
	if _, err := Bootstrap(workDir, config.Config{Provider: "openrouter", Model: "gpt-4.1"}); err != nil {
		t.Fatalf("Bootstrap() error = %v", err)
	}
	s := store.New(workDir)
	draft := writeStoreFile(t, s, "drafts/ch01/draft.md", "# Draft\n")
	review := writeStoreFile(t, s, "reviews/ch01/review.json", `{"passed":true}`)
	cp := checkpoint.Checkpoint{
		Step:      7,
		Phase:     "review_chapter",
		CreatedAt: recoverFixedTime(),
		Outputs: []checkpoint.OutputArtifact{
			{Kind: "draft", Path: "drafts/ch01/draft.md", SHA256: draft.SHA256},
			{Kind: "review", Path: "reviews/ch01/review.json", SHA256: review.SHA256},
		},
		NextExpected: "commit_chapter",
	}
	progress := contracts.Progress{
		Phase:             "review_chapter",
		Status:            "running",
		CurrentChapter:    "ch01",
		CompletedChapters: []string{},
		PendingChapters:   []string{"ch01"},
		UpdatedAt:         recoverFixedTime(),
	}
	if err := checkpoint.Record(s, cp, progress); err != nil {
		t.Fatalf("Record() error = %v", err)
	}
	draftBefore := readFile(t, s.Path("drafts", "ch01", "draft.md"))
	reviewBefore := readFile(t, s.Path("reviews", "ch01", "review.json"))

	result, err := recoverAt(workDir, recoverFixedTime().Add(time.Minute))
	if err != nil {
		t.Fatalf("Recover() error = %v", err)
	}
	if !result.OK {
		t.Fatalf("result = %#v", result)
	}
	if result.ResumedFrom != 7 || !result.RunUpdated {
		t.Fatalf("result metadata = %#v", result)
	}
	if !strings.Contains(result.RecoveryPrompt, "next_expected: commit_chapter") {
		t.Fatalf("prompt missing next_expected:\n%s", result.RecoveryPrompt)
	}
	if !strings.Contains(result.RecoveryPrompt, "do not overwrite") {
		t.Fatalf("prompt missing overwrite guard:\n%s", result.RecoveryPrompt)
	}
	if !bytes.Equal(draftBefore, readFile(t, s.Path("drafts", "ch01", "draft.md"))) {
		t.Fatalf("Recover() changed draft artifact")
	}
	if !bytes.Equal(reviewBefore, readFile(t, s.Path("reviews", "ch01", "review.json"))) {
		t.Fatalf("Recover() changed review artifact")
	}

	var run contracts.Run
	if err := store.ReadJSON(s.RunPath(), &run); err != nil {
		t.Fatalf("ReadJSON(run) error = %v", err)
	}
	if run.ResumedFrom == nil || *run.ResumedFrom != 7 {
		t.Fatalf("run.resumed_from = %#v", run.ResumedFrom)
	}
	if len(run.Events) != 1 || run.Events[0].Kind != "recover" {
		t.Fatalf("run events = %#v", run.Events)
	}
}

func TestRecoverReportsValidationFailureWithoutTouchingRun(t *testing.T) {
	workDir := t.TempDir()
	if _, err := Bootstrap(workDir, config.Config{}); err != nil {
		t.Fatalf("Bootstrap() error = %v", err)
	}
	s := store.New(workDir)

	result, err := recoverAt(workDir, recoverFixedTime())
	if err != nil {
		t.Fatalf("Recover() error = %v", err)
	}
	if result.OK {
		t.Fatalf("Recover() unexpectedly OK")
	}
	if !containsPart(result.Errors, "latest checkpoint does not exist") {
		t.Fatalf("errors = %#v", result.Errors)
	}

	var run contracts.Run
	if err := store.ReadJSON(s.RunPath(), &run); err != nil {
		t.Fatalf("ReadJSON(run) error = %v", err)
	}
	if run.ResumedFrom != nil {
		t.Fatalf("run.resumed_from = %#v", run.ResumedFrom)
	}
}

func writeStoreFile(t *testing.T, s store.Store, relPath, data string) store.WriteResult {
	t.Helper()
	result, err := store.WriteFile(s.Path(filepath.FromSlash(relPath)), []byte(data), store.CreateOnly)
	if err != nil {
		t.Fatalf("WriteFile(%s) error = %v", relPath, err)
	}
	return result
}

func containsPart(values []string, part string) bool {
	for _, value := range values {
		if strings.Contains(value, part) {
			return true
		}
	}
	return false
}

func recoverFixedTime() time.Time {
	return time.Date(2026, 6, 7, 12, 0, 0, 0, time.UTC)
}
