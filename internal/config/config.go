package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

const (
	GlobalDirName  = ".aipaper"
	ConfigFileName = "config.json"
	ProjectFile    = "aipaper.json"
)

type Config struct {
	Provider        string                    `json:"provider,omitempty"`
	Model           string                    `json:"model,omitempty"`
	DefaultLanguage string                    `json:"default_language,omitempty"`
	CitationStyle   string                    `json:"citation_style,omitempty"`
	Style           string                    `json:"style,omitempty"`
	Providers       map[string]ProviderConfig `json:"providers,omitempty"`
	Roles           map[string]RoleConfig     `json:"roles,omitempty"`
}

type ProviderConfig struct {
	Type    string         `json:"type,omitempty"`
	APIKey  string         `json:"api_key,omitempty"`
	BaseURL string         `json:"base_url,omitempty"`
	Models  []string       `json:"models,omitempty"`
	Extra   map[string]any `json:"extra,omitempty"`
}

type RoleConfig struct {
	Provider string  `json:"provider,omitempty"`
	Model    string  `json:"model,omitempty"`
	MaxTurns int     `json:"max_turns,omitempty"`
	Temp     float64 `json:"temperature,omitempty"`
}

type LoadOptions struct {
	WorkDir    string
	ConfigPath string
}

func GlobalPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, GlobalDirName, ConfigFileName), nil
}

func ProjectPath(workDir string) string {
	if workDir == "" {
		workDir = "."
	}
	return filepath.Join(workDir, ProjectFile)
}

func Load(opts LoadOptions) (Config, []string, error) {
	var merged Config
	var loaded []string

	globalPath, err := GlobalPath()
	if err != nil {
		return Config{}, nil, err
	}
	for _, path := range []string{globalPath, ProjectPath(opts.WorkDir), opts.ConfigPath} {
		if path == "" {
			continue
		}
		cfg, ok, err := read(path)
		if err != nil {
			return Config{}, loaded, err
		}
		if !ok {
			continue
		}
		merged = Merge(merged, cfg)
		loaded = append(loaded, path)
	}
	return merged, loaded, nil
}

func Merge(base, override Config) Config {
	out := base
	if override.Provider != "" {
		out.Provider = override.Provider
	}
	if override.Model != "" {
		out.Model = override.Model
	}
	if override.DefaultLanguage != "" {
		out.DefaultLanguage = override.DefaultLanguage
	}
	if override.CitationStyle != "" {
		out.CitationStyle = override.CitationStyle
	}
	if override.Style != "" {
		out.Style = override.Style
	}
	out.Providers = mergeMap(out.Providers, override.Providers, MergeProvider)
	out.Roles = mergeMap(out.Roles, override.Roles, MergeRole)
	return out
}

func MergeProvider(base, override ProviderConfig) ProviderConfig {
	out := base
	if override.Type != "" {
		out.Type = override.Type
	}
	if override.APIKey != "" {
		out.APIKey = override.APIKey
	}
	if override.BaseURL != "" {
		out.BaseURL = override.BaseURL
	}
	if len(override.Models) > 0 {
		out.Models = append([]string(nil), override.Models...)
	}
	if len(override.Extra) > 0 {
		if out.Extra == nil {
			out.Extra = map[string]any{}
		}
		for k, v := range override.Extra {
			out.Extra[k] = v
		}
	}
	return out
}

func MergeRole(base, override RoleConfig) RoleConfig {
	out := base
	if override.Provider != "" {
		out.Provider = override.Provider
	}
	if override.Model != "" {
		out.Model = override.Model
	}
	if override.MaxTurns != 0 {
		out.MaxTurns = override.MaxTurns
	}
	if override.Temp != 0 {
		out.Temp = override.Temp
	}
	return out
}

func (c Config) Validate() error {
	if c.Provider == "" && c.Model == "" {
		return nil
	}
	if c.Provider == "" {
		return errors.New("provider is required when model is set")
	}
	if c.Model == "" {
		return errors.New("model is required when provider is set")
	}
	if c.Providers != nil {
		if _, ok := c.Providers[c.Provider]; !ok && c.Provider != "" {
			return fmt.Errorf("provider %q is not configured", c.Provider)
		}
	}
	return nil
}

func read(path string) (Config, bool, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return Config{}, false, nil
	}
	if err != nil {
		return Config{}, false, err
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return Config{}, false, fmt.Errorf("parse %s: %w", path, err)
	}
	return cfg, true, nil
}

func mergeMap[T any](base, override map[string]T, merge func(T, T) T) map[string]T {
	if len(base) == 0 && len(override) == 0 {
		return nil
	}
	out := map[string]T{}
	for k, v := range base {
		out[k] = v
	}
	for k, v := range override {
		if existing, ok := out[k]; ok {
			out[k] = merge(existing, v)
			continue
		}
		out[k] = v
	}
	return out
}
