package store

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestStorePathsLayoutAndRelativePaths(t *testing.T) {
	workDir := t.TempDir()
	s := New(workDir)

	wantRoot := filepath.Join(workDir, "output", "aipaper")
	if s.Root() != wantRoot {
		t.Fatalf("Root() = %q, want %q", s.Root(), wantRoot)
	}
	if got := s.Rel("checkpoints", "latest.json"); got != "checkpoints/latest.json" {
		t.Fatalf("Rel() = %q", got)
	}
	if got := StepCheckpointName(7); got != "step-000007.json" {
		t.Fatalf("StepCheckpointName(7) = %q", got)
	}
	if got := StepCheckpointName(-1); got != "step-000000.json" {
		t.Fatalf("StepCheckpointName(-1) = %q", got)
	}

	if err := EnsureLayout(s); err != nil {
		t.Fatalf("EnsureLayout() error = %v", err)
	}
	for _, dir := range RequiredDirs() {
		info, err := os.Stat(s.Path(dir))
		if err != nil {
			t.Fatalf("stat %s: %v", dir, err)
		}
		if !info.IsDir() {
			t.Fatalf("%s is not a directory", dir)
		}
	}
}

func TestWriteFileCreateOnlyIsIdempotentAndDetectsConflict(t *testing.T) {
	path := filepath.Join(t.TempDir(), "artifact.txt")
	first, err := WriteFile(path, []byte("same"), CreateOnly)
	if err != nil {
		t.Fatalf("first WriteFile() error = %v", err)
	}
	if first.AlreadyExists {
		t.Fatalf("first write unexpectedly marked AlreadyExists")
	}

	second, err := WriteFile(path, []byte("same"), CreateOnly)
	if err != nil {
		t.Fatalf("second WriteFile() error = %v", err)
	}
	if !second.AlreadyExists || second.SHA256 != first.SHA256 {
		t.Fatalf("second result = %#v, first = %#v", second, first)
	}

	_, err = WriteFile(path, []byte("different"), CreateOnly)
	if err == nil || !strings.Contains(err.Error(), "artifact conflict") {
		t.Fatalf("conflicting WriteFile() error = %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if string(data) != "same" {
		t.Fatalf("conflict changed artifact content to %q", data)
	}
}

func TestWriteJSONAndReadJSONRejectInvalidShapes(t *testing.T) {
	type sample struct {
		Name string `json:"name"`
	}
	path := filepath.Join(t.TempDir(), "sample.json")

	if _, err := WriteJSON(path, sample{Name: "ok"}, CreateOnly); err != nil {
		t.Fatalf("WriteJSON() error = %v", err)
	}
	var got sample
	if err := ReadJSON(path, &got); err != nil {
		t.Fatalf("ReadJSON(valid) error = %v", err)
	}
	if got.Name != "ok" {
		t.Fatalf("decoded = %#v", got)
	}

	writeRaw(t, path, `{"name":"ok","extra":true}`)
	if err := ReadJSON(path, &got); err == nil || !strings.Contains(err.Error(), "unknown field") {
		t.Fatalf("ReadJSON(unknown field) error = %v", err)
	}

	writeRaw(t, path, `{"name":"ok"} {"name":"again"}`)
	if err := ReadJSON(path, &got); err == nil || !strings.Contains(err.Error(), "trailing JSON") {
		t.Fatalf("ReadJSON(trailing value) error = %v", err)
	}
}

func writeRaw(t *testing.T, path, data string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
}
