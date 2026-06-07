package app

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/voocel/agentcore"
	agentllm "github.com/voocel/agentcore/llm"
	"github.com/voocel/litellm"

	aipagent "github.com/thisxiaoyuQAQ/aipaper-cli/internal/agent"
	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/config"
	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/contracts"
	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/store"
)

const defaultCoordinatorMaxTurns = 12

type AgentRuntime struct {
	Agent        *agentcore.Agent
	Provider     string
	Model        string
	SystemPrompt string
	Tools        []agentcore.Tool
	Store        store.Store
}

type AgentRuntimeOptions struct {
	WorkDir        string
	Config         config.Config
	RecoveryPrompt string
	Model          agentcore.ChatModel
	Writer         aipagent.WriterRunner
	EventSink      func(contracts.RunEvent)
	Now            func() time.Time
}

type RoleRuntimeConfig struct {
	Role            string
	Provider        string
	RuntimeProvider string
	Model           string
	MaxTurns        int
	Temperature     float64
	ProviderConfig  config.ProviderConfig
	APIKey          string
}

func NewAgentRuntime(opts AgentRuntimeOptions) (*AgentRuntime, error) {
	roleCfg, err := ResolveRoleRuntime(opts.Config, aipagent.RoleCoordinator)
	if err != nil {
		return nil, err
	}
	model := opts.Model
	if model == nil {
		model, err = NewChatModel(roleCfg)
		if err != nil {
			return nil, err
		}
	}

	s := store.New(opts.WorkDir)
	tools := aipagent.DefaultTools(s, opts.Writer)
	systemPrompt := aipagent.CoordinatorSystemPrompt(aipagent.PromptOptions{RecoveryPrompt: opts.RecoveryPrompt})
	agent := agentcore.NewAgent(
		agentcore.WithModel(model),
		agentcore.WithSystemPrompt(systemPrompt),
		agentcore.WithTools(tools...),
		agentcore.WithMaxTurns(roleCfg.MaxTurns),
	)
	if opts.EventSink != nil {
		now := opts.Now
		if now == nil {
			now = func() time.Time { return time.Now().UTC() }
		}
		agent.Subscribe(func(ev agentcore.Event) {
			opts.EventSink(aipagent.ProjectEvent(ev, now()))
		})
	}

	return &AgentRuntime{
		Agent:        agent,
		Provider:     roleCfg.Provider,
		Model:        roleCfg.Model,
		SystemPrompt: systemPrompt,
		Tools:        tools,
		Store:        s,
	}, nil
}

func ResolveRoleRuntime(cfg config.Config, role string) (RoleRuntimeConfig, error) {
	roleCfg := cfg.Roles[role]
	providerName := firstNonEmpty(roleCfg.Provider, cfg.Provider)
	modelName := firstNonEmpty(roleCfg.Model, cfg.Model)
	if providerName == "" {
		return RoleRuntimeConfig{}, errors.New("provider is required for agent runtime")
	}
	if modelName == "" {
		return RoleRuntimeConfig{}, errors.New("model is required for agent runtime")
	}
	providerCfg, ok := cfg.Providers[providerName]
	if len(cfg.Providers) > 0 && !ok {
		return RoleRuntimeConfig{}, fmt.Errorf("provider %q is not configured", providerName)
	}
	apiKey, err := ResolveSecret(providerCfg.APIKey)
	if err != nil {
		return RoleRuntimeConfig{}, err
	}
	maxTurns := roleCfg.MaxTurns
	if maxTurns <= 0 {
		maxTurns = defaultCoordinatorMaxTurns
	}
	return RoleRuntimeConfig{
		Role:            role,
		Provider:        providerName,
		RuntimeProvider: runtimeProviderName(providerName, providerCfg),
		Model:           modelName,
		MaxTurns:        maxTurns,
		Temperature:     roleCfg.Temp,
		ProviderConfig:  providerCfg,
		APIKey:          apiKey,
	}, nil
}

func NewChatModel(roleCfg RoleRuntimeConfig) (agentcore.ChatModel, error) {
	providerCfg := LiteLLMProviderConfig(roleCfg.ProviderConfig, roleCfg.APIKey)
	modelOpts := []agentllm.ModelOption{}
	if providerCfg.APIKey != "" {
		modelOpts = append(modelOpts, agentllm.WithAPIKey(providerCfg.APIKey))
	}
	if providerCfg.BaseURL != "" {
		modelOpts = append(modelOpts, agentllm.WithBaseURL(providerCfg.BaseURL))
	}
	return agentllm.NewModel(roleCfg.RuntimeProvider, roleCfg.Model, modelOpts...)
}

func LiteLLMProviderConfig(provider config.ProviderConfig, apiKey string) litellm.ProviderConfig {
	extra := map[string]any{}
	for k, v := range provider.Extra {
		extra[k] = v
	}
	return litellm.ProviderConfig{
		APIKey:  apiKey,
		BaseURL: provider.BaseURL,
		Extra:   extra,
	}
}

func ResolveSecret(value string) (string, error) {
	if !strings.HasPrefix(value, "env:") {
		return value, nil
	}
	name := strings.TrimSpace(strings.TrimPrefix(value, "env:"))
	if name == "" {
		return "", errors.New("environment variable name is required")
	}
	resolved := os.Getenv(name)
	if resolved == "" {
		return "", fmt.Errorf("environment variable %s is required", name)
	}
	return resolved, nil
}

func runtimeProviderName(name string, provider config.ProviderConfig) string {
	if provider.Type != "" && provider.Type != "openai-compatible" {
		return provider.Type
	}
	return name
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
