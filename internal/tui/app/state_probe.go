package app

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/agent"
	runtimeapp "github.com/thisxiaoyuQAQ/aipaper-cli/internal/app"
	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/checkpoint"
	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/config"
	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/contracts"
	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/store"
	requirementstui "github.com/thisxiaoyuQAQ/aipaper-cli/internal/tui/requirements"
)

type StateProbeFunc func(workDir string) (ProbeResult, error)

type ProbeResult struct {
	WorkDir          string
	ConfigOK         bool
	ConfigError      error
	HasRequirements  bool
	HasMaterials     bool
	HasCandidates    bool
	HasConfirmedRefs bool
	HasAcceptedWork  bool
	HasFinalOutputs  bool
	HasRun           bool
	CheckpointValid  bool
	CheckpointStep   int
	SuggestedScreen  Screen

	CheckpointPhase        string
	CheckpointNextExpected string
	CheckpointErrors       []string
	ProgressStatus         string

	// Quality engine fields
	HasQualityArtifacts bool
	QualityMode         string
}

func StateProbe(workDir string) (ProbeResult, error) {
	if workDir == "" {
		workDir = "."
	}
	s := store.New(workDir)
	result := ProbeResult{WorkDir: workDir}

	cfg, loaded, err := config.Load(config.LoadOptions{WorkDir: workDir})
	if err != nil {
		result.ConfigError = err
		result.SuggestedScreen = ScreenConfigWizard
		return result, nil
	}
	if len(loaded) == 0 {
		result.ConfigError = errors.New("config file not found")
		result.SuggestedScreen = ScreenConfigWizard
		return result, nil
	}
	if err := validateTUIRuntimeConfig(cfg); err != nil {
		result.ConfigError = err
		result.SuggestedScreen = ScreenConfigWizard
		return result, nil
	}
	result.ConfigOK = true

	validation := checkpoint.ValidateLatest(s)
	if validation.OK {
		result.CheckpointValid = true
		result.CheckpointStep = validation.Step
		result.CheckpointPhase = validation.Phase
		result.CheckpointNextExpected = validation.Next
	} else if !onlyMissingLatest(validation.Errors) {
		result.CheckpointErrors = append([]string(nil), validation.Errors...)
	}

	result.HasRequirements = hasRequirements(workDir, s)
	result.HasMaterials = hasMaterials(s)
	result.HasCandidates = hasCandidates(s)
	result.HasConfirmedRefs = hasConfirmedReferences(s)
	result.HasAcceptedWork = hasAcceptedWork(s)
	result.HasFinalOutputs = hasFinalOutputs(s)
	result.HasRun = hasRun(s)
	result.HasQualityArtifacts = hasQualityArtifacts(s)
	result.QualityMode = detectQualityMode(workDir, s)
	if result.HasFinalOutputs {
		result.HasAcceptedWork = true
	}
	result.ProgressStatus = progressStatus(workDir)

	if result.CheckpointValid && result.HasRun && runIsIncomplete(result.ProgressStatus, result.HasFinalOutputs) {
		result.SuggestedScreen = ScreenRecoverPrompt
		return result, nil
	}
	if !result.HasRequirements {
		result.SuggestedScreen = ScreenRequirements
		return result, nil
	}
	if !result.HasConfirmedRefs {
		if result.HasCandidates {
			result.SuggestedScreen = ScreenReferences
		} else if result.HasMaterials {
			result.SuggestedScreen = ScreenSearchProgress
		} else {
			result.SuggestedScreen = ScreenMaterialsScan
		}
		return result, nil
	}
	if result.HasFinalOutputs {
		result.SuggestedScreen = ScreenDone
		return result, nil
	}
	if !result.HasAcceptedWork {
		result.SuggestedScreen = ScreenWriting
		return result, nil
	}
	if !result.HasFinalOutputs {
		result.SuggestedScreen = ScreenExportSummary
		return result, nil
	}
	result.SuggestedScreen = ScreenDone
	return result, nil
}

func validateTUIRuntimeConfig(cfg config.Config) error {
	if err := cfg.Validate(); err != nil {
		return err
	}
	for _, role := range []string{agent.RoleCoordinator, agent.RoleArchitect, agent.RoleWriter, agent.RoleEditor} {
		if _, err := runtimeapp.ResolveRoleRuntime(cfg, role); err != nil {
			return fmt.Errorf("runtime config for role %s is invalid: %w", role, err)
		}
	}
	return nil
}

func hasRequirements(workDir string, s store.Store) bool {
	var requirements contracts.Requirements
	if err := store.ReadJSON(s.RequirementsPath(), &requirements); err != nil {
		return false
	}
	if strings.TrimSpace(requirements.MaterialDir) != "" && !filepath.IsAbs(requirements.MaterialDir) {
		requirements.MaterialDir = filepath.Join(workDir, requirements.MaterialDir)
	}
	return requirementstui.Validate(requirements) == nil
}

func hasMaterials(s store.Store) bool {
	var manifest contracts.MaterialManifest
	if err := store.ReadJSON(s.Path("materials", "manifest.json"), &manifest); err != nil {
		return false
	}
	for _, item := range manifest.Items {
		if item.Status != "parsed" {
			continue
		}
		if item.OutputText == "" || item.OutputMeta == "" {
			continue
		}
		if !fileExists(s.Path(filepath.FromSlash(item.OutputText))) {
			continue
		}
		if !fileExists(s.Path(filepath.FromSlash(item.OutputMeta))) {
			continue
		}
		return true
	}
	return false
}

func hasCandidates(s store.Store) bool {
	var candidates contracts.ReferenceCandidates
	if err := store.ReadJSON(s.Path("references", "candidates.json"), &candidates); err != nil {
		return false
	}
	return len(candidates.Items) > 0
}

func hasConfirmedReferences(s store.Store) bool {
	var confirmed contracts.ConfirmedReferences
	if err := store.ReadJSON(s.Path("references", "confirmed.json"), &confirmed); err != nil {
		return false
	}
	return len(confirmed.Items) > 0
}

func hasRun(s store.Store) bool {
	var run contracts.Run
	if err := store.ReadJSON(s.RunPath(), &run); err != nil {
		return false
	}
	return strings.TrimSpace(run.RunID) != ""
}

func hasAcceptedWork(s store.Store) bool {
	entries, err := os.ReadDir(s.Path("drafts"))
	if err != nil {
		return false
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		if fileExists(s.Path("drafts", entry.Name(), "accepted.md")) {
			return true
		}
	}
	return false
}

func hasFinalOutputs(s store.Store) bool {
	for _, rel := range []string{
		"final/paper.md",
		"final/references.md",
		"final/citation-trace.json",
		"final/report.md",
	} {
		if !fileExists(s.Path(filepath.FromSlash(rel))) {
			return false
		}
	}
	return true
}

func progressStatus(workDir string) string {
	progress, ok, err := runtimeapp.LoadProgress(workDir)
	if err != nil || !ok {
		return ""
	}
	return strings.TrimSpace(progress.Status)
}

func runIsIncomplete(progressStatus string, hasFinalOutputs bool) bool {
	if hasFinalOutputs {
		return false
	}
	switch strings.ToLower(strings.TrimSpace(progressStatus)) {
	case "completed", "complete", "done", "finished":
		return false
	default:
		return true
	}
}

func onlyMissingLatest(errors []string) bool {
	return len(errors) == 1 && strings.Contains(errors[0], "latest checkpoint does not exist")
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func hasQualityArtifacts(s store.Store) bool {
	// Check for quality/ directory artifacts
	qualityPaths := []string{
		"quality/evidence-table.json",
		"quality/section-quality-plan.json",
		"quality/claim-graph.json",
	}
	for _, rel := range qualityPaths {
		if fileExists(s.Path(filepath.FromSlash(rel))) {
			return true
		}
	}
	return false
}

func detectQualityMode(workDir string, s store.Store) string {
	var req contracts.Requirements
	if err := store.ReadJSON(s.RequirementsPath(), &req); err != nil {
		return ""
	}
	mode := strings.TrimSpace(req.QualityMode)
	if mode == "" {
		// Default for new runs when requirements exist but no quality_mode set
		return "enhanced"
	}
	return mode
}
