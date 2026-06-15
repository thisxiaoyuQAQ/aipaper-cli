package configwizard

import (
	"strings"
	"testing"

	runtimeapp "github.com/thisxiaoyuQAQ/aipaper-cli/internal/app"
	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/config"
)

func TestTemplateDefaults(t *testing.T) {
	tests := []struct {
		name       string
		wantType   string
		wantURL    string
		wantModel  string
		wantAPIKey string
	}{
		{name: "OpenAI", wantType: "openai", wantURL: "https://api.openai.com/v1", wantModel: "gpt-5.5", wantAPIKey: "env:OPENAI_API_KEY"},
		{name: "Anthropic", wantType: "anthropic", wantURL: "https://api.anthropic.com", wantModel: "claude-opus-4-8", wantAPIKey: "env:ANTHROPIC_API_KEY"},
		{name: "Ollama", wantType: "ollama", wantURL: "http://localhost:11434", wantModel: "llama3", wantAPIKey: ""},
		{name: "Custom", wantType: "", wantURL: "", wantModel: "", wantAPIKey: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			model := NewModel(Options{}).SelectTemplate(tt.name)
			if model.Err() != nil {
				t.Fatalf("SelectTemplate() error = %v", model.Err())
			}
			if model.FieldValue(FieldProviderType) != tt.wantType {
				t.Fatalf("type = %q, want %q", model.FieldValue(FieldProviderType), tt.wantType)
			}
			if model.FieldValue(FieldBaseURL) != tt.wantURL {
				t.Fatalf("base URL = %q, want %q", model.FieldValue(FieldBaseURL), tt.wantURL)
			}
			if model.FieldValue(FieldModel) != tt.wantModel {
				t.Fatalf("model = %q, want %q", model.FieldValue(FieldModel), tt.wantModel)
			}
			if model.FieldValue(FieldAPIKey) != tt.wantAPIKey {
				t.Fatalf("api key = %q, want %q", model.FieldValue(FieldAPIKey), tt.wantAPIKey)
			}
		})
	}
}

func TestSaveOpenAIConfigIsLoadableAndRuntimeResolvable(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "secret-openai-key")
	workDir := t.TempDir()
	model := NewModel(Options{WorkDir: workDir}).SelectTemplate("OpenAI")

	cfg, err := model.Config()
	if err != nil {
		t.Fatalf("Config() error = %v", err)
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
	path, err := config.SaveProject(workDir, cfg)
	if err != nil {
		t.Fatalf("SaveProject() error = %v", err)
	}
	loaded, _, err := config.Load(config.LoadOptions{WorkDir: workDir})
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if loaded.Provider != "default" || loaded.Model != "gpt-5.5" {
		t.Fatalf("loaded config = %#v", loaded)
	}
	if loaded.Providers["default"].APIKey != "env:OPENAI_API_KEY" {
		t.Fatalf("loaded api key = %q", loaded.Providers["default"].APIKey)
	}
	for _, role := range []string{"coordinator", "writer"} {
		roleCfg, err := runtimeapp.ResolveRoleRuntime(loaded, role)
		if err != nil {
			t.Fatalf("ResolveRoleRuntime(%s) error = %v", role, err)
		}
		if roleCfg.Provider != "default" || roleCfg.Model != "gpt-5.5" || roleCfg.APIKey != "secret-openai-key" {
			t.Fatalf("role %s config = %#v", role, roleCfg)
		}
	}
	if !strings.HasSuffix(path, config.ProjectFile) {
		t.Fatalf("saved path = %q", path)
	}
}

func TestOllamaAllowsEmptyAPIKey(t *testing.T) {
	model := NewModel(Options{}).SelectTemplate("Ollama")
	cfg, err := model.Config()
	if err != nil {
		t.Fatalf("Config() error = %v", err)
	}
	if cfg.Providers["default"].APIKey != "" {
		t.Fatalf("api key = %q, want empty", cfg.Providers["default"].APIKey)
	}
	if _, err := runtimeapp.ResolveRoleRuntime(cfg, "writer"); err != nil {
		t.Fatalf("ResolveRoleRuntime(writer) error = %v", err)
	}
}

func TestCustomConfigCanBeCompleted(t *testing.T) {
	model := NewModel(Options{}).SelectTemplate("Custom")
	model = model.SetField(FieldProviderType, "openai-compatible")
	model = model.SetField(FieldBaseURL, "https://llm.example/v1")
	model = model.SetField(FieldModel, "custom-model")
	model = model.SetField(FieldAPIKey, "env:CUSTOM_LLM_API_KEY")

	cfg, err := model.Config()
	if err != nil {
		t.Fatalf("Config() error = %v", err)
	}
	provider := cfg.Providers["default"]
	if provider.Type != "openai-compatible" || provider.BaseURL != "https://llm.example/v1" || cfg.Model != "custom-model" {
		t.Fatalf("custom config = %#v", cfg)
	}
}

func TestCustomConfigRequiresBaseURL(t *testing.T) {
	model := NewModel(Options{}).SelectTemplate("Custom")
	model = model.SetField(FieldProviderType, "openai-compatible")
	model = model.SetField(FieldModel, "custom-model")

	_, err := model.Config()
	if err == nil || !strings.Contains(err.Error(), "基础地址") {
		t.Fatalf("Config() error = %v, want 基础地址 error", err)
	}
}

func TestSummaryAndViewMaskDirectAPIKey(t *testing.T) {
	secret := "secret-api-key"
	model := NewModel(Options{}).SelectTemplate("OpenAI")
	model = model.SetField(FieldAPIKey, secret)

	if strings.Contains(model.Summary(), secret) {
		t.Fatalf("summary leaked api key: %s", model.Summary())
	}
	if strings.Contains(model.View(), secret) {
		t.Fatalf("view leaked api key: %s", model.View())
	}
	if !model.DirectSecretWarning() {
		t.Fatalf("DirectSecretWarning() = false, want true")
	}
}

func TestUpdateSavesAndMarksDone(t *testing.T) {
	var saved config.Config
	model := NewModel(Options{
		WorkDir: "work",
		Save: func(workDir string, cfg config.Config) (string, error) {
			if workDir != "work" {
				t.Fatalf("workDir = %q, want work", workDir)
			}
			saved = cfg
			return "work/aipaper.json", nil
		},
	})

	model = model.UpdateKey("enter")
	if model.Step() != StepFields {
		t.Fatalf("step = %v, want fields", model.Step())
	}
	model = model.UpdateKey("enter")
	if model.Step() != StepSummary {
		t.Fatalf("step = %v, want summary", model.Step())
	}
	model = model.UpdateKey("enter")
	if !model.Done() || model.SavedPath() != "work/aipaper.json" {
		t.Fatalf("done = %v, path = %q", model.Done(), model.SavedPath())
	}
	if saved.Provider != "default" || saved.Model == "" {
		t.Fatalf("saved config = %#v", saved)
	}
}
