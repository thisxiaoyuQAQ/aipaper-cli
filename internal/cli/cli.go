package cli

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/app"
	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/config"
	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/store"
)

const helpText = `aipaper-cli

Usage:
  aipaper-cli init [--workdir DIR] [--config FILE]
  aipaper-cli status [--workdir DIR]
  aipaper-cli recover [--workdir DIR]
  aipaper-cli config [--workdir DIR] [--config FILE]

Milestone 1 provides the storage, config, and checkpoint foundation. The TUI,
agent loop, material parsing, search, and export flows are implemented later.
`

func Run(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	_ = ctx
	if len(args) == 0 {
		fmt.Fprint(stdout, helpText)
		return 0
	}
	switch args[0] {
	case "help", "-h", "--help":
		fmt.Fprint(stdout, helpText)
		return 0
	case "init":
		return runInit(args[1:], stdout, stderr)
	case "status":
		return runStatus(args[1:], stdout, stderr)
	case "recover":
		return runRecover(args[1:], stdout, stderr)
	case "config":
		return runConfig(args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "unknown command %q\n\n%s", args[0], helpText)
		return 2
	}
}

func runInit(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("init", flag.ContinueOnError)
	fs.SetOutput(stderr)
	workDir := fs.String("workdir", ".", "project working directory")
	configPath := fs.String("config", "", "extra config file")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	cfg, loaded, err := config.Load(config.LoadOptions{WorkDir: *workDir, ConfigPath: *configPath})
	if err != nil {
		fmt.Fprintf(stderr, "load config: %v\n", err)
		return 1
	}
	if err := cfg.Validate(); err != nil {
		fmt.Fprintf(stderr, "config: %v\n", err)
		return 1
	}
	result, err := app.Bootstrap(*workDir, cfg)
	if err != nil {
		fmt.Fprintf(stderr, "init store: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "store: %s\n", clean(result.StoreRoot))
	if len(loaded) > 0 {
		fmt.Fprintf(stdout, "config: %s\n", strings.Join(cleanAll(loaded), ", "))
	}
	if len(result.Created) > 0 {
		fmt.Fprintf(stdout, "created: %s\n", strings.Join(cleanAll(result.Created), ", "))
	}
	if len(result.Existing) > 0 {
		fmt.Fprintf(stdout, "existing: %s\n", strings.Join(cleanAll(result.Existing), ", "))
	}
	return 0
}

func runStatus(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("status", flag.ContinueOnError)
	fs.SetOutput(stderr)
	workDir := fs.String("workdir", ".", "project working directory")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	progress, ok, err := app.LoadProgress(*workDir)
	if err != nil {
		fmt.Fprintf(stderr, "read progress: %v\n", err)
		return 1
	}
	if !ok {
		fmt.Fprintf(stdout, "status: not initialized\nstore: %s\n", clean(store.New(*workDir).Root()))
		return 0
	}
	writePretty(stdout, progress)
	return 0
}

func runRecover(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("recover", flag.ContinueOnError)
	fs.SetOutput(stderr)
	workDir := fs.String("workdir", ".", "project working directory")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	result, err := app.Recover(*workDir)
	if err != nil {
		fmt.Fprintf(stderr, "recover: %v\n", err)
		return 1
	}
	writePretty(stdout, result)
	if !result.OK {
		return 1
	}
	return 0
}

func runConfig(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("config", flag.ContinueOnError)
	fs.SetOutput(stderr)
	workDir := fs.String("workdir", ".", "project working directory")
	configPath := fs.String("config", "", "extra config file")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	cfg, loaded, err := config.Load(config.LoadOptions{WorkDir: *workDir, ConfigPath: *configPath})
	if err != nil {
		fmt.Fprintf(stderr, "load config: %v\n", err)
		return 1
	}
	redactConfig(&cfg)
	out := struct {
		Loaded []string      `json:"loaded"`
		Config config.Config `json:"config"`
	}{Loaded: cleanAll(loaded), Config: cfg}
	writePretty(stdout, out)
	return 0
}

func writePretty(w io.Writer, value any) {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		fmt.Fprintf(w, `{"error":%q}`+"\n", err.Error())
		return
	}
	fmt.Fprintln(w, string(data))
}

func redactConfig(cfg *config.Config) {
	for name, provider := range cfg.Providers {
		if provider.APIKey != "" {
			provider.APIKey = "redacted"
			cfg.Providers[name] = provider
		}
	}
}

func cleanAll(paths []string) []string {
	out := make([]string, 0, len(paths))
	for _, path := range paths {
		out = append(out, clean(path))
	}
	return out
}

func clean(path string) string {
	if path == "" {
		return path
	}
	return filepath.Clean(path)
}
