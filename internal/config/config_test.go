package config

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestMergeDeepMergesProviderAndRoleConfig(t *testing.T) {
	base := Config{
		Provider:        "openrouter",
		Model:           "base-model",
		UILanguage:      "zh-CN",
		DefaultLanguage: "zh-CN",
		Providers: map[string]ProviderConfig{
			"openrouter": {
				Type:    "openai-compatible",
				APIKey:  "env:OPENROUTER_API_KEY",
				BaseURL: "https://openrouter.ai/api/v1",
				Models:  []string{"base-model"},
				Extra:   map[string]any{"timeout": float64(30), "base": true},
			},
		},
		Roles: map[string]RoleConfig{
			"writer": {Model: "base-writer", MaxTurns: 3},
		},
	}
	override := Config{
		Model:      "override-model",
		UILanguage: "en",
		Providers: map[string]ProviderConfig{
			"openrouter": {
				Models: []string{"override-model"},
				Extra:  map[string]any{"timeout": float64(60), "retries": float64(2)},
			},
		},
		Roles: map[string]RoleConfig{
			"writer": {Provider: "openrouter", Temp: 0.2},
		},
	}

	merged := Merge(base, override)

	if merged.Provider != "openrouter" || merged.Model != "override-model" || merged.UILanguage != "en" || merged.DefaultLanguage != "zh-CN" {
		t.Fatalf("unexpected top-level merge: %#v", merged)
	}
	provider := merged.Providers["openrouter"]
	if provider.Type != "openai-compatible" || provider.APIKey != "env:OPENROUTER_API_KEY" || provider.BaseURL == "" {
		t.Fatalf("provider fields were not preserved: %#v", provider)
	}
	if !reflect.DeepEqual(provider.Models, []string{"override-model"}) {
		t.Fatalf("models = %#v", provider.Models)
	}
	wantExtra := map[string]any{"timeout": float64(60), "base": true, "retries": float64(2)}
	if !reflect.DeepEqual(provider.Extra, wantExtra) {
		t.Fatalf("extra = %#v, want %#v", provider.Extra, wantExtra)
	}
	if base.Providers["openrouter"].Extra["timeout"] != float64(30) {
		t.Fatalf("MergeProvider mutated base extra: %#v", base.Providers["openrouter"].Extra)
	}

	role := merged.Roles["writer"]
	if role.Provider != "openrouter" || role.Model != "base-writer" || role.MaxTurns != 3 || role.Temp != 0.2 {
		t.Fatalf("role merge = %#v", role)
	}
}

func TestValidateRequiresProviderModelPairAndConfiguredProvider(t *testing.T) {
	tests := []struct {
		name    string
		cfg     Config
		wantErr string
	}{
		{name: "empty ok", cfg: Config{}},
		{name: "model needs provider", cfg: Config{Model: "gpt"}, wantErr: "provider is required"},
		{name: "provider needs model", cfg: Config{Provider: "openrouter"}, wantErr: "model is required"},
		{
			name: "provider must exist",
			cfg: Config{
				Provider:  "missing",
				Model:     "gpt",
				Providers: map[string]ProviderConfig{"openrouter": {}},
			},
			wantErr: "not configured",
		},
		{
			name: "configured provider ok",
			cfg: Config{
				Provider:  "openrouter",
				Model:     "gpt",
				Providers: map[string]ProviderConfig{"openrouter": {}},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("Validate() error = %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("Validate() error = %v, want containing %q", err, tt.wantErr)
			}
		})
	}
}

func TestLoadMergesGlobalProjectAndExplicitConfigInOrder(t *testing.T) {
	home := t.TempDir()
	workDir := t.TempDir()
	setHome(t, home)

	globalPath := filepath.Join(home, GlobalDirName, ConfigFileName)
	writeFile(t, globalPath, `{
  "provider": "openrouter",
  "model": "global-model",
  "providers": {
    "openrouter": {
      "type": "openai-compatible",
      "api_key": "global-key",
      "base_url": "https://global.example"
    }
  },
  "roles": {
    "writer": {
      "model": "global-writer",
      "max_turns": 3
    }
  }
}`)
	projectPath := ProjectPath(workDir)
	writeFile(t, projectPath, `{
  "model": "project-model",
  "providers": {
    "openrouter": {
      "base_url": "https://project.example"
    }
  }
}`)
	explicitPath := filepath.Join(t.TempDir(), "explicit.json")
	writeFile(t, explicitPath, `{
  "default_language": "zh-CN",
  "roles": {
    "writer": {
      "temperature": 0.2
    }
  }
}`)

	cfg, loaded, err := Load(LoadOptions{WorkDir: workDir, ConfigPath: explicitPath})
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	wantLoaded := []string{globalPath, projectPath, explicitPath}
	if !reflect.DeepEqual(loaded, wantLoaded) {
		t.Fatalf("loaded = %#v, want %#v", loaded, wantLoaded)
	}
	if cfg.Provider != "openrouter" || cfg.Model != "project-model" || cfg.DefaultLanguage != "zh-CN" || cfg.UILanguage != "zh-CN" {
		t.Fatalf("unexpected merged config: %#v", cfg)
	}
	provider := cfg.Providers["openrouter"]
	if provider.APIKey != "global-key" || provider.BaseURL != "https://project.example" {
		t.Fatalf("provider = %#v", provider)
	}
	role := cfg.Roles["writer"]
	if role.Model != "global-writer" || role.MaxTurns != 3 || role.Temp != 0.2 {
		t.Fatalf("role = %#v", role)
	}
}

func TestLoadReturnsParseErrors(t *testing.T) {
	home := t.TempDir()
	setHome(t, home)
	explicitPath := filepath.Join(t.TempDir(), "bad.json")
	writeFile(t, explicitPath, `{`)

	_, _, err := Load(LoadOptions{WorkDir: t.TempDir(), ConfigPath: explicitPath})
	if err == nil || !strings.Contains(err.Error(), "parse") {
		t.Fatalf("Load() error = %v, want parse error", err)
	}
}

func TestSaveProjectWritesLoadableConfig(t *testing.T) {
	home := t.TempDir()
	workDir := t.TempDir()
	setHome(t, home)
	cfg := Config{
		Provider:        "default",
		Model:           "gpt-test",
		DefaultLanguage: "zh-CN",
		CitationStyle:   "gbt7714",
		Providers: map[string]ProviderConfig{
			"default": {Type: "openai", APIKey: "env:OPENAI_API_KEY", BaseURL: "https://api.openai.com/v1"},
		},
	}

	path, err := SaveProject(workDir, cfg)
	if err != nil {
		t.Fatalf("SaveProject() error = %v", err)
	}
	if path != ProjectPath(workDir) {
		t.Fatalf("path = %q, want %q", path, ProjectPath(workDir))
	}
	loaded, paths, err := Load(LoadOptions{WorkDir: workDir})
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if !reflect.DeepEqual(paths, []string{path}) {
		t.Fatalf("loaded paths = %#v, want %#v", paths, []string{path})
	}
	if loaded.Provider != cfg.Provider || loaded.Model != cfg.Model || loaded.UILanguage != "zh-CN" {
		t.Fatalf("loaded config = %#v", loaded)
	}
}

func TestLoadCanonicalizesUILanguageAliases(t *testing.T) {
	home := t.TempDir()
	workDir := t.TempDir()
	setHome(t, home)
	writeFile(t, ProjectPath(workDir), `{"ui_language":"english"}`)

	loaded, _, err := Load(LoadOptions{WorkDir: workDir})
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if loaded.UILanguage != "en" {
		t.Fatalf("UILanguage = %q, want en", loaded.UILanguage)
	}
}

func TestValidateRejectsInvalidUILanguage(t *testing.T) {
	err := Config{UILanguage: "fr"}.Validate()
	if err == nil || !strings.Contains(err.Error(), "ui_language") {
		t.Fatalf("Validate() error = %v, want ui_language error", err)
	}
}

func TestRedactAndMaskSecretDoNotExposeAPIKeys(t *testing.T) {
	cfg := Config{Providers: map[string]ProviderConfig{
		"default": {APIKey: "secret-api-key"},
		"env":     {APIKey: "env:OPENAI_API_KEY"},
	}}

	redacted := Redact(cfg)
	if redacted.Providers["default"].APIKey != "redacted" || redacted.Providers["env"].APIKey != "redacted" {
		t.Fatalf("redacted config = %#v", redacted.Providers)
	}
	if cfg.Providers["default"].APIKey != "secret-api-key" {
		t.Fatalf("Redact mutated source config")
	}
	if got := MaskSecret("secret-api-key"); got == "" || strings.Contains(got, "secret-api-key") {
		t.Fatalf("MaskSecret() = %q", got)
	}
	if got := MaskSecret("env:OPENAI_API_KEY"); got != "env:OPENAI_API_KEY" {
		t.Fatalf("MaskSecret(env) = %q", got)
	}
}

func setHome(t *testing.T, home string) {
	t.Helper()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
}

func writeFile(t *testing.T, path, data string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
}
