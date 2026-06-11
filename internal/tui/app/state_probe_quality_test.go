package app

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/config"
	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/contracts"
	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/store"
)

func createTestConfig(dir string) error {
	// Create config in HOME directory (global config)
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	configDir := filepath.Join(homeDir, ".aipaper")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		return err
	}
	cfg := config.Config{
		Provider: "openai",
		Model:    "gpt-4",
		Providers: map[string]config.ProviderConfig{
			"openai": {
				Type:    "openai",
				BaseURL: "https://api.openai.com/v1",
				APIKey:  "test-key",
			},
		},
		Roles: map[string]config.RoleConfig{
			"coordinator": {Provider: "openai", Model: "gpt-4"},
			"architect":   {Provider: "openai", Model: "gpt-4"},
			"writer":      {Provider: "openai", Model: "gpt-4"},
			"editor":      {Provider: "openai", Model: "gpt-4"},
		},
	}
	configPath := filepath.Join(configDir, "config.json")
	_, err = store.WriteJSON(configPath, cfg, store.Overwrite)
	return err
}

func TestStateProbe_DetectsQualityArtifacts(t *testing.T) {
	dir := t.TempDir()
	s := store.New(dir)

	// Create test config
	if err := createTestConfig(dir); err != nil {
		t.Fatal(err)
	}

	// Ensure store directories exist
	if err := store.EnsureLayout(s); err != nil {
		t.Fatal(err)
	}

	// Create minimal requirements
	materialsDir := filepath.Join(dir, "materials")
	req := contracts.Requirements{
		Topic:       "test",
		Language:    "en",
		CitationStyle: "apa",
		TargetWords: 1000,
		MaterialDir: materialsDir,
		QualityMode: "enhanced",
	}
	if err := os.MkdirAll(materialsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := store.WriteJSON(s.RequirementsPath(), req, store.Overwrite); err != nil {
		t.Fatal(err)
	}

	// Create quality artifacts
	qualityDir := s.Path("quality")
	if err := os.MkdirAll(qualityDir, 0o755); err != nil {
		t.Fatal(err)
	}
	evidenceTablePath := filepath.Join(qualityDir, "evidence-table.json")
	if err := os.WriteFile(evidenceTablePath, []byte(`{"generated_at":"2026-01-01T00:00:00Z","items":[]}`), 0o644); err != nil {
		t.Fatal(err)
	}

	probe, err := StateProbe(dir)
	if err != nil {
		t.Fatalf("StateProbe failed: %v", err)
	}

	if !probe.HasQualityArtifacts {
		t.Errorf("Expected HasQualityArtifacts=true, got false")
	}

	if probe.QualityMode != "enhanced" {
		t.Errorf("Expected QualityMode=enhanced, got %q", probe.QualityMode)
	}
}

func TestStateProbe_QualityMode_DefaultsToEnhanced(t *testing.T) {
	dir := t.TempDir()
	s := store.New(dir)

	// Create test config
	if err := createTestConfig(dir); err != nil {
		t.Fatal(err)
	}

	// Ensure store directories exist
	if err := store.EnsureLayout(s); err != nil {
		t.Fatal(err)
	}

	// Create requirements without quality_mode field
	materialsDir := filepath.Join(dir, "materials")
	req := contracts.Requirements{
		Topic:       "test",
		Language:    "en",
		CitationStyle: "apa",
		TargetWords: 1000,
		MaterialDir: materialsDir,
		// QualityMode intentionally omitted
	}
	if err := os.MkdirAll(materialsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := store.WriteJSON(s.RequirementsPath(), req, store.Overwrite); err != nil {
		t.Fatal(err)
	}

	probe, err := StateProbe(dir)
	if err != nil {
		t.Fatalf("StateProbe failed: %v", err)
	}

	if probe.QualityMode != "enhanced" {
		t.Errorf("Expected QualityMode to default to 'enhanced', got %q", probe.QualityMode)
	}
}

func TestStateProbe_NoQualityArtifacts_CompatibilityMode(t *testing.T) {
	dir := t.TempDir()
	s := store.New(dir)

	// Create test config
	if err := createTestConfig(dir); err != nil {
		t.Fatal(err)
	}

	// Ensure store directories exist
	if err := store.EnsureLayout(s); err != nil {
		t.Fatal(err)
	}

	// Create requirements with enhanced mode
	materialsDir := filepath.Join(dir, "materials")
	req := contracts.Requirements{
		Topic:       "test",
		Language:    "en",
		CitationStyle: "apa",
		TargetWords: 1000,
		MaterialDir: materialsDir,
		QualityMode: "enhanced",
	}
	if err := os.MkdirAll(materialsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := store.WriteJSON(s.RequirementsPath(), req, store.Overwrite); err != nil {
		t.Fatal(err)
	}

	// No quality artifacts created

	probe, err := StateProbe(dir)
	if err != nil {
		t.Fatalf("StateProbe failed: %v", err)
	}

	if probe.HasQualityArtifacts {
		t.Errorf("Expected HasQualityArtifacts=false, got true")
	}

	// Mode should still be detected from requirements
	if probe.QualityMode != "enhanced" {
		t.Errorf("Expected QualityMode=enhanced, got %q", probe.QualityMode)
	}
}

func TestStateProbe_QualityMode_AllModes(t *testing.T) {
	modes := []string{"fast", "enhanced", "strict"}

	for _, mode := range modes {
		t.Run(mode, func(t *testing.T) {
			dir := t.TempDir()
			s := store.New(dir)

			// Create test config
			if err := createTestConfig(dir); err != nil {
				t.Fatal(err)
			}

			// Ensure store directories exist
			if err := store.EnsureLayout(s); err != nil {
				t.Fatal(err)
			}

			materialsDir := filepath.Join(dir, "materials")
			req := contracts.Requirements{
				Topic:       "test",
				Language:    "en",
				CitationStyle: "apa",
				TargetWords: 1000,
				MaterialDir: materialsDir,
				QualityMode: mode,
			}
			if err := os.MkdirAll(materialsDir, 0o755); err != nil {
				t.Fatal(err)
			}
			if _, err := store.WriteJSON(s.RequirementsPath(), req, store.Overwrite); err != nil {
				t.Fatal(err)
			}

			probe, err := StateProbe(dir)
			if err != nil {
				t.Fatalf("StateProbe failed: %v", err)
			}

			if probe.QualityMode != mode {
				t.Errorf("Expected QualityMode=%s, got %q", mode, probe.QualityMode)
			}
		})
	}
}
