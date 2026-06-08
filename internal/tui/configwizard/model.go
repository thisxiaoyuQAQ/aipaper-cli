package configwizard

import (
	"fmt"
	"strings"

	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/agent"
	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/config"
)

type Step int

const (
	StepTemplate Step = iota
	StepFields
	StepSummary
)

type Field int

const (
	FieldProviderName Field = iota
	FieldProviderType
	FieldBaseURL
	FieldModel
	FieldAPIKey
	FieldDefaultLanguage
	FieldCitationStyle
)

type ProviderTemplate struct {
	Name           string
	Type           string
	BaseURL        string
	Model          string
	APIKeyEnv      string
	APIKeyRequired bool
}

type Options struct {
	WorkDir string
	Save    func(string, config.Config) (string, error)
}

type Model struct {
	workDir      string
	save         func(string, config.Config) (string, error)
	step         Step
	template     ProviderTemplate
	templateIdx  int
	cursor       int
	values       map[Field]string
	done         bool
	canceled     bool
	err          error
	savedPath    string
	directSecret bool
}

var orderedFields = []Field{
	FieldProviderName,
	FieldProviderType,
	FieldBaseURL,
	FieldModel,
	FieldAPIKey,
	FieldDefaultLanguage,
	FieldCitationStyle,
}

func NewModel(opts Options) Model {
	save := opts.Save
	if save == nil {
		save = config.SaveProject
	}
	m := Model{
		workDir: opts.WorkDir,
		save:    save,
		step:    StepTemplate,
		values:  map[Field]string{},
	}
	m.applyTemplate(DefaultTemplates()[0])
	m.step = StepTemplate
	return m
}

func DefaultTemplates() []ProviderTemplate {
	return []ProviderTemplate{
		{Name: "OpenAI", Type: "openai", BaseURL: "https://api.openai.com/v1", Model: "gpt-5.5", APIKeyEnv: "OPENAI_API_KEY", APIKeyRequired: true},
		{Name: "Anthropic", Type: "anthropic", BaseURL: "https://api.anthropic.com", Model: "claude-opus-4-8", APIKeyEnv: "ANTHROPIC_API_KEY", APIKeyRequired: true},
		{Name: "Ollama", Type: "ollama", BaseURL: "http://localhost:11434", Model: "llama3"},
		{Name: "Custom", APIKeyEnv: "CUSTOM_LLM_API_KEY"},
	}
}

func (m Model) UpdateKey(key string) Model {
	key = normalizeKey(key)
	if m.done || m.canceled {
		return m
	}
	switch key {
	case "q", "ctrl+c", "esc":
		m.canceled = true
		return m
	}
	switch m.step {
	case StepTemplate:
		return m.updateTemplate(key)
	case StepFields:
		return m.updateFields(key)
	case StepSummary:
		return m.updateSummary(key)
	default:
		m.err = fmt.Errorf("unknown wizard step")
		return m
	}
}

func (m Model) SelectTemplate(name string) Model {
	for i, template := range DefaultTemplates() {
		if strings.EqualFold(template.Name, strings.TrimSpace(name)) {
			m.templateIdx = i
			m.applyTemplate(template)
			m.err = nil
			return m
		}
	}
	m.err = fmt.Errorf("unknown provider template")
	return m
}

func (m Model) SetField(field Field, value string) Model {
	if m.values == nil {
		m.values = map[Field]string{}
	}
	m.values[field] = value
	m.updateDirectSecretWarning()
	return m
}

func (m Model) Config() (config.Config, error) {
	if err := m.validateFields(); err != nil {
		return config.Config{}, err
	}
	providerName := strings.TrimSpace(m.values[FieldProviderName])
	modelName := strings.TrimSpace(m.values[FieldModel])
	provider := config.ProviderConfig{
		Type:    strings.TrimSpace(m.values[FieldProviderType]),
		APIKey:  strings.TrimSpace(m.values[FieldAPIKey]),
		BaseURL: strings.TrimSpace(m.values[FieldBaseURL]),
	}
	if modelName != "" {
		provider.Models = []string{modelName}
	}
	roles := map[string]config.RoleConfig{}
	for _, role := range []string{agent.RoleCoordinator, agent.RoleArchitect, agent.RoleWriter, agent.RoleEditor} {
		roles[role] = config.RoleConfig{Provider: providerName, Model: modelName}
	}
	return config.Config{
		Provider:        providerName,
		Model:           modelName,
		DefaultLanguage: strings.TrimSpace(m.values[FieldDefaultLanguage]),
		CitationStyle:   strings.TrimSpace(m.values[FieldCitationStyle]),
		Providers: map[string]config.ProviderConfig{
			providerName: provider,
		},
		Roles: roles,
	}, nil
}

func (m Model) Summary() string {
	cfg, err := m.Config()
	if err != nil {
		return "Configuration is incomplete."
	}
	provider := cfg.Providers[cfg.Provider]
	apiKey := config.MaskSecret(provider.APIKey)
	if apiKey == "" {
		apiKey = "(empty)"
	}
	return fmt.Sprintf(
		"Provider: %s\nType: %s\nBase URL: %s\nModel: %s\nAPI key: %s\nLanguage: %s\nCitation: %s",
		cfg.Provider,
		provider.Type,
		provider.BaseURL,
		cfg.Model,
		apiKey,
		cfg.DefaultLanguage,
		cfg.CitationStyle,
	)
}

func (m Model) Step() Step {
	return m.step
}

func (m Model) CurrentField() Field {
	if m.cursor < 0 || m.cursor >= len(orderedFields) {
		return FieldProviderName
	}
	return orderedFields[m.cursor]
}

func (m Model) FieldValue(field Field) string {
	return m.values[field]
}

func (m Model) SelectedTemplate() ProviderTemplate {
	return m.template
}

func (m Model) Done() bool {
	return m.done
}

func (m Model) Canceled() bool {
	return m.canceled
}

func (m Model) Err() error {
	return m.err
}

func (m Model) SavedPath() string {
	return m.savedPath
}

func (m Model) DirectSecretWarning() bool {
	return m.directSecret
}

func (m Model) updateTemplate(key string) Model {
	templates := DefaultTemplates()
	switch key {
	case "up", "k":
		if m.templateIdx > 0 {
			m.templateIdx--
		}
		m.applyTemplate(templates[m.templateIdx])
	case "down", "j":
		if m.templateIdx < len(templates)-1 {
			m.templateIdx++
		}
		m.applyTemplate(templates[m.templateIdx])
	case "enter":
		m.step = StepFields
		m.cursor = 0
		m.err = nil
	}
	return m
}

func (m Model) updateFields(key string) Model {
	switch key {
	case "up", "shift+tab":
		if m.cursor > 0 {
			m.cursor--
		}
	case "down", "tab":
		if m.cursor < len(orderedFields)-1 {
			m.cursor++
		}
	case "space":
		m.appendRune(' ')
	case "backspace":
		m.backspace()
	case "enter":
		if err := m.validateFields(); err != nil {
			m.err = err
			return m
		}
		m.step = StepSummary
		m.err = nil
	default:
		if len([]rune(key)) == 1 {
			m.appendRune([]rune(key)[0])
		}
	}
	return m
}

func (m Model) updateSummary(key string) Model {
	switch key {
	case "b", "backspace":
		m.step = StepFields
		m.err = nil
	case "enter":
		cfg, err := m.Config()
		if err != nil {
			m.err = err
			return m
		}
		path, err := m.save(m.workDir, cfg)
		if err != nil {
			m.err = fmt.Errorf("save config failed: %w", err)
			return m
		}
		m.savedPath = path
		m.done = true
		m.err = nil
	}
	return m
}

func (m *Model) applyTemplate(template ProviderTemplate) {
	m.template = template
	apiKey := ""
	if template.APIKeyRequired && template.APIKeyEnv != "" {
		apiKey = "env:" + template.APIKeyEnv
	}
	m.values = map[Field]string{
		FieldProviderName:    "default",
		FieldProviderType:    template.Type,
		FieldBaseURL:         template.BaseURL,
		FieldModel:           template.Model,
		FieldAPIKey:          apiKey,
		FieldDefaultLanguage: "zh-CN",
		FieldCitationStyle:   "gbt7714",
	}
	m.directSecret = false
}

func (m Model) validateFields() error {
	if strings.TrimSpace(m.values[FieldProviderName]) == "" {
		return fmt.Errorf("provider name is required")
	}
	if strings.TrimSpace(m.values[FieldProviderType]) == "" {
		return fmt.Errorf("provider type is required")
	}
	if strings.EqualFold(m.template.Name, "Custom") && strings.TrimSpace(m.values[FieldBaseURL]) == "" {
		return fmt.Errorf("base url is required for custom providers")
	}
	if strings.TrimSpace(m.values[FieldModel]) == "" {
		return fmt.Errorf("model is required")
	}
	apiKey := strings.TrimSpace(m.values[FieldAPIKey])
	if m.template.APIKeyRequired && apiKey == "" {
		return fmt.Errorf("api key is required; use an env: reference when possible")
	}
	if lang := strings.TrimSpace(m.values[FieldDefaultLanguage]); lang == "" {
		return fmt.Errorf("default language is required")
	} else if lang != "zh-CN" && lang != "en" {
		return fmt.Errorf("default language must be zh-CN or en")
	}
	if style := strings.TrimSpace(m.values[FieldCitationStyle]); style == "" {
		return fmt.Errorf("citation style is required")
	} else if style != "gbt7714" && style != "apa" {
		return fmt.Errorf("citation style must be gbt7714 or apa")
	}
	return nil
}

func (m *Model) appendRune(r rune) {
	field := m.CurrentField()
	m.values[field] += string(r)
	m.updateDirectSecretWarning()
}

func (m *Model) backspace() {
	field := m.CurrentField()
	runes := []rune(m.values[field])
	if len(runes) == 0 {
		return
	}
	m.values[field] = string(runes[:len(runes)-1])
	m.updateDirectSecretWarning()
}

func (m *Model) updateDirectSecretWarning() {
	apiKey := strings.TrimSpace(m.values[FieldAPIKey])
	m.directSecret = apiKey != "" && !strings.HasPrefix(apiKey, "env:")
}

func normalizeKey(key string) string {
	if key == " " {
		return "space"
	}
	return strings.ToLower(strings.TrimSpace(key))
}
