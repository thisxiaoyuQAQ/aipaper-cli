package app

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/voocel/agentcore"

	aipagent "github.com/thisxiaoyuQAQ/aipaper-cli/internal/agent"
	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/config"
	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/contracts"
)

func TestResolveRoleRuntimeUsesRoleOverrideAndEnvSecret(t *testing.T) {
	t.Setenv("OPENROUTER_API_KEY", "secret-key")
	cfg := config.Config{
		Provider: "openrouter",
		Model:    "default-model",
		Providers: map[string]config.ProviderConfig{
			"openrouter": {
				Type:    "openai-compatible",
				APIKey:  "env:OPENROUTER_API_KEY",
				BaseURL: "https://openrouter.ai/api/v1",
				Extra:   map[string]any{"trace": true},
			},
		},
		Roles: map[string]config.RoleConfig{
			aipagent.RoleCoordinator: {Model: "coordinator-model", MaxTurns: 5, Temp: 0.2},
		},
	}

	got, err := ResolveRoleRuntime(cfg, aipagent.RoleCoordinator)
	if err != nil {
		t.Fatalf("ResolveRoleRuntime() error = %v", err)
	}
	if got.Provider != "openrouter" || got.RuntimeProvider != "openrouter" || got.Model != "coordinator-model" {
		t.Fatalf("role config = %#v", got)
	}
	if got.APIKey != "secret-key" || got.MaxTurns != 5 || got.Temperature != 0.2 {
		t.Fatalf("role details = %#v", got)
	}

	providerCfg := LiteLLMProviderConfig(got.ProviderConfig, got.APIKey)
	if providerCfg.APIKey != "secret-key" || providerCfg.BaseURL != "https://openrouter.ai/api/v1" {
		t.Fatalf("litellm provider config = %#v", providerCfg)
	}
	providerCfg.Extra["trace"] = false
	if got.ProviderConfig.Extra["trace"] != true {
		t.Fatalf("LiteLLMProviderConfig mutated source extra")
	}
}

func TestResolveSecretReportsMissingEnv(t *testing.T) {
	_, err := ResolveSecret("env:DOES_NOT_EXIST_FOR_AIPAPER_TEST")
	if err == nil || !strings.Contains(err.Error(), "DOES_NOT_EXIST_FOR_AIPAPER_TEST") {
		t.Fatalf("ResolveSecret() error = %v", err)
	}
}

func TestNewAgentRuntimeUsesInjectedModelAndDoesNotCreateNetworkClient(t *testing.T) {
	cfg := config.Config{
		Provider: "openai",
		Model:    "gpt-test",
		Providers: map[string]config.ProviderConfig{
			"openai": {APIKey: "test-key"},
		},
	}
	var events []string
	runtime, err := NewAgentRuntime(AgentRuntimeOptions{
		WorkDir:        t.TempDir(),
		Config:         cfg,
		RecoveryPrompt: "resume from step 3",
		Model:          fakeChatModel{},
		Writer:         aipagent.WriterRunnerFunc(func(context.Context, json.RawMessage) (any, error) { return nil, nil }),
		EventSink: func(contracts.RunEvent) {
			events = append(events, "called")
		},
		Now: func() time.Time {
			return time.Date(2026, 6, 7, 12, 0, 0, 0, time.UTC)
		},
	})
	if err != nil {
		t.Fatalf("NewAgentRuntime() error = %v", err)
	}
	if runtime.Agent == nil || runtime.Provider != "openai" || runtime.Model != "gpt-test" {
		t.Fatalf("runtime = %#v", runtime)
	}
	if len(runtime.Tools) != 14 {
		t.Fatalf("tools = %d", len(runtime.Tools))
	}
	toolNames := map[string]bool{}
	for _, tool := range runtime.Tools {
		toolNames[tool.Name()] = true
	}
	if !toolNames["save_evidence_table"] || !toolNames["save_section_quality_plan"] || !toolNames["save_claim_graph"] || !toolNames["save_verification_result"] || !toolNames["quality_gate_check"] {
		t.Fatalf("quality tools missing, names = %#v", toolNames)
	}
	if !strings.Contains(runtime.SystemPrompt, "resume from step 3") {
		t.Fatalf("system prompt = %s", runtime.SystemPrompt)
	}
	_ = events
}

type fakeChatModel struct{}

func (fakeChatModel) Generate(context.Context, []agentcore.Message, []agentcore.ToolSpec, ...agentcore.CallOption) (*agentcore.LLMResponse, error) {
	return nil, errors.New("unused")
}

func (fakeChatModel) GenerateStream(context.Context, []agentcore.Message, []agentcore.ToolSpec, ...agentcore.CallOption) (<-chan agentcore.StreamEvent, error) {
	return nil, errors.New("unused")
}

func (fakeChatModel) SupportsTools() bool {
	return true
}
