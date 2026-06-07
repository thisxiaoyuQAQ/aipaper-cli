package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/checkpoint"
	configpkg "github.com/thisxiaoyuQAQ/aipaper-cli/internal/config"
	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/contracts"
	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/store"
)

func TestInitCommandCreatesLayoutAndIsIdempotent(t *testing.T) {
	setIsolatedHome(t)
	workDir := t.TempDir()

	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"init", "--workdir", workDir}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("first init code = %d, stderr = %s", code, stderr.String())
	}
	s := store.New(workDir)
	for _, dir := range store.RequiredDirs() {
		info, err := os.Stat(s.Path(dir))
		if err != nil {
			t.Fatalf("stat %s: %v", dir, err)
		}
		if !info.IsDir() {
			t.Fatalf("%s is not a directory", dir)
		}
	}
	runBefore := mustRead(t, s.RunPath())
	progressBefore := mustRead(t, s.ProgressPath())

	stdout.Reset()
	stderr.Reset()
	code = Run(context.Background(), []string{"init", "--workdir", workDir}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("second init code = %d, stderr = %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "existing:") {
		t.Fatalf("second init output did not report existing files: %s", stdout.String())
	}
	if !bytes.Equal(runBefore, mustRead(t, s.RunPath())) {
		t.Fatalf("second init changed run.json")
	}
	if !bytes.Equal(progressBefore, mustRead(t, s.ProgressPath())) {
		t.Fatalf("second init changed progress.json")
	}
}

func TestConfigCommandRedactsAPIKeys(t *testing.T) {
	setIsolatedHome(t)
	workDir := t.TempDir()
	configPath := filepath.Join(t.TempDir(), "config.json")
	writeText(t, configPath, `{
  "provider": "openrouter",
  "model": "gpt-4.1",
  "providers": {
    "openrouter": {
      "type": "openai-compatible",
      "api_key": "secret-api-key",
      "base_url": "https://openrouter.ai/api/v1"
    }
  }
}`)

	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"config", "--workdir", workDir, "--config", configPath}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("config code = %d, stderr = %s", code, stderr.String())
	}
	if strings.Contains(stdout.String(), "secret-api-key") {
		t.Fatalf("config output leaked API key: %s", stdout.String())
	}
	if !strings.Contains(stdout.String(), "redacted") {
		t.Fatalf("config output did not redact API key: %s", stdout.String())
	}

	var out struct {
		Loaded []string         `json:"loaded"`
		Config configpkg.Config `json:"config"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &out); err != nil {
		t.Fatalf("config output is not JSON: %v\n%s", err, stdout.String())
	}
	if out.Config.Providers["openrouter"].APIKey != "redacted" {
		t.Fatalf("redacted provider = %#v", out.Config.Providers["openrouter"])
	}
}

func TestStatusCommandReportsNotInitialized(t *testing.T) {
	workDir := t.TempDir()
	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"status", "--workdir", workDir}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("status code = %d, stderr = %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "status: not initialized") {
		t.Fatalf("status output = %s", stdout.String())
	}
}

func TestRecoverCommandOutputsPromptAndUpdatesRun(t *testing.T) {
	setIsolatedHome(t)
	workDir := t.TempDir()
	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"init", "--workdir", workDir}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("init code = %d, stderr = %s", code, stderr.String())
	}
	s := store.New(workDir)
	artifact, err := store.WriteFile(s.Path("drafts", "ch01", "draft.md"), []byte("# Draft\n"), store.CreateOnly)
	if err != nil {
		t.Fatalf("WriteFile(draft) error = %v", err)
	}
	if err := checkpoint.Record(s, checkpoint.Checkpoint{
		Step:      7,
		Phase:     "draft_chapter",
		CreatedAt: fixedCLITime(),
		Outputs: []checkpoint.OutputArtifact{{
			Kind:   "draft",
			Path:   "drafts/ch01/draft.md",
			SHA256: artifact.SHA256,
		}},
		NextExpected: "review_chapter",
	}, contracts.Progress{
		Phase:             "draft_chapter",
		Status:            "running",
		CurrentChapter:    "ch01",
		CompletedChapters: []string{},
		PendingChapters:   []string{"ch01"},
		UpdatedAt:         fixedCLITime(),
	}); err != nil {
		t.Fatalf("Record() error = %v", err)
	}

	stdout.Reset()
	stderr.Reset()
	code = Run(context.Background(), []string{"recover", "--workdir", workDir}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("recover code = %d, stderr = %s, stdout = %s", code, stderr.String(), stdout.String())
	}
	var out struct {
		OK             bool     `json:"ok"`
		ResumedFrom    int      `json:"resumed_from"`
		Checked        []string `json:"checked"`
		RecoveryPrompt string   `json:"recovery_prompt"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &out); err != nil {
		t.Fatalf("recover output is not JSON: %v\n%s", err, stdout.String())
	}
	if !out.OK || out.ResumedFrom != 7 || len(out.Checked) != 1 {
		t.Fatalf("recover output = %#v", out)
	}
	if !strings.Contains(out.RecoveryPrompt, "next_expected: review_chapter") {
		t.Fatalf("recover prompt = %s", out.RecoveryPrompt)
	}

	var run contracts.Run
	if err := store.ReadJSON(s.RunPath(), &run); err != nil {
		t.Fatalf("ReadJSON(run) error = %v", err)
	}
	if run.ResumedFrom == nil || *run.ResumedFrom != 7 {
		t.Fatalf("run.resumed_from = %#v", run.ResumedFrom)
	}
}

func TestRecoverCommandReturnsNonZeroForInvalidCheckpoint(t *testing.T) {
	setIsolatedHome(t)
	workDir := t.TempDir()
	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"init", "--workdir", workDir}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("init code = %d, stderr = %s", code, stderr.String())
	}

	stdout.Reset()
	stderr.Reset()
	code = Run(context.Background(), []string{"recover", "--workdir", workDir}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("recover code = %d, stderr = %s, stdout = %s", code, stderr.String(), stdout.String())
	}
	var out struct {
		OK     bool     `json:"ok"`
		Errors []string `json:"errors"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &out); err != nil {
		t.Fatalf("recover output is not JSON: %v\n%s", err, stdout.String())
	}
	if out.OK || !containsError(out.Errors, "latest checkpoint does not exist") {
		t.Fatalf("recover output = %#v", out)
	}
}

func setIsolatedHome(t *testing.T) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
}

func mustRead(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	return data
}

func writeText(t *testing.T, path, data string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
}

func containsError(errors []string, part string) bool {
	for _, err := range errors {
		if strings.Contains(err, part) {
			return true
		}
	}
	return false
}

func fixedCLITime() time.Time {
	return time.Date(2026, 6, 7, 12, 0, 0, 0, time.UTC)
}
