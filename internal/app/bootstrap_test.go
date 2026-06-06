package app

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/config"
	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/store"
)

func TestBootstrapCreatesLayoutAndKeepsExistingState(t *testing.T) {
	workDir := t.TempDir()
	cfg := config.Config{Provider: "openrouter", Model: "gpt-4.1"}

	first, err := Bootstrap(workDir, cfg)
	if err != nil {
		t.Fatalf("first Bootstrap() error = %v", err)
	}
	if len(first.Created) != 2 || len(first.Existing) != 0 {
		t.Fatalf("first result = %#v", first)
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

	runBefore := readFile(t, s.RunPath())
	progressBefore := readFile(t, s.ProgressPath())

	second, err := Bootstrap(workDir, config.Config{Provider: "other", Model: "other-model"})
	if err != nil {
		t.Fatalf("second Bootstrap() error = %v", err)
	}
	if len(second.Created) != 0 || len(second.Existing) != 2 {
		t.Fatalf("second result = %#v", second)
	}
	if !bytes.Equal(runBefore, readFile(t, s.RunPath())) {
		t.Fatalf("second Bootstrap() changed run.json")
	}
	if !bytes.Equal(progressBefore, readFile(t, s.ProgressPath())) {
		t.Fatalf("second Bootstrap() changed progress.json")
	}
}

func TestBootstrapRejectsCorruptExistingState(t *testing.T) {
	workDir := t.TempDir()
	s := store.New(workDir)
	if err := store.EnsureLayout(s); err != nil {
		t.Fatalf("EnsureLayout() error = %v", err)
	}
	if err := os.WriteFile(s.RunPath(), []byte(`{"run_id":"bad","unknown":true}`), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	_, err := Bootstrap(workDir, config.Config{})
	if err == nil {
		t.Fatalf("Bootstrap() succeeded with corrupt run.json")
	}
}

func readFile(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	return data
}
