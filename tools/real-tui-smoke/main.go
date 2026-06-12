// Package main is a throwaway real-provider TUI-path smoke for
// BUG-20260613-01 step 4 (verification). It drives the EXACT production
// entry the TUI uses — tuiapp.StartWritingRuntime -> NewAgentRuntime with the
// real role runners -> kickoff/continuation loop -> event bridge — against a
// real LLM, over the quality-mini fixtures, and then exports the final
// artifacts. Only the interactive rendering (keyboard/screen) is skipped;
// that part is the user's manual Windows acceptance.
//
// The API key is injected via env only (SMOKE_API_KEY); aipaper.json written
// here references it as "env:SMOKE_API_KEY" and never contains the secret.
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	runtimeapp "github.com/thisxiaoyuQAQ/aipaper-cli/internal/app"
	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/config"
	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/contracts"
	finalexport "github.com/thisxiaoyuQAQ/aipaper-cli/internal/export"
	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/materials"
	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/quality"
	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/references"
	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/store"
	tuiapp "github.com/thisxiaoyuQAQ/aipaper-cli/internal/tui/app"
	writingtui "github.com/thisxiaoyuQAQ/aipaper-cli/internal/tui/writing"
)

func main() {
	if err := run(); err != nil {
		fmt.Println("FATAL:", err)
		os.Exit(1)
	}
}

func run() error {
	if os.Getenv("SMOKE_API_KEY") == "" || os.Getenv("SMOKE_BASE_URL") == "" || os.Getenv("SMOKE_MODEL") == "" {
		return fmt.Errorf("SMOKE_API_KEY / SMOKE_BASE_URL / SMOKE_MODEL must be set in the environment")
	}

	workDir := filepath.Join("agent-output", "real-tui-smoke")
	if err := os.RemoveAll(workDir); err != nil {
		return err
	}
	if err := os.MkdirAll(workDir, 0o755); err != nil {
		return err
	}

	// Project config exactly as the ConfigWizard would write it: the key is an
	// env reference, never the secret itself.
	cfg := config.Config{
		Provider: "custom",
		Model:    os.Getenv("SMOKE_MODEL"),
		Providers: map[string]config.ProviderConfig{
			"custom": {Type: "openai", APIKey: "env:SMOKE_API_KEY", BaseURL: os.Getenv("SMOKE_BASE_URL")},
		},
	}
	if _, err := config.SaveProject(workDir, cfg); err != nil {
		return err
	}
	if _, err := runtimeapp.Bootstrap(workDir, cfg); err != nil {
		return err
	}
	s := store.New(workDir)

	// Requirements: short two-chapter mini review, enhanced quality mode.
	req := contracts.Requirements{
		Topic:             "Behavioral and digital treatment of insomnia",
		ResearchQuestions: []string{"How durable are CBT-I effects across delivery modes?"},
		Scope:             "short mini review, two chapters",
		Language:          "en",
		CitationStyle:     "APA",
		QualityMode:       quality.ModeEnhanced,
		TargetWords:       300,
		MaterialDir:       "fixtures/quality-mini/materials",
		AllowOnlineSearch: false,
	}
	if _, err := store.WriteJSON(s.RequirementsPath(), req, store.Overwrite); err != nil {
		return err
	}

	// Materials scan + reference confirmation, same as the TUI screens do.
	matResult, err := materials.ProcessDir(req.MaterialDir, s)
	if err != nil {
		return err
	}
	candidates := contracts.ReferenceCandidates{
		Items: references.AssignCandidateIDs(references.DedupeCandidates(matResult.Candidates), 1),
	}
	if _, err := store.WriteJSON(s.Path("references", "candidates.json"), candidates, store.Overwrite); err != nil {
		return err
	}
	var confirmIDs []string
	for _, c := range candidates.Items {
		if strings.Contains(c.DOI, "wearables-sleep") {
			continue // stays unconfirmed, matching the established comparison setup
		}
		confirmIDs = append(confirmIDs, c.ID)
	}
	confirmation, err := references.ConfirmCandidates(s, candidates, references.ConfirmationDecision{
		ConfirmedIDs: confirmIDs, ConfirmedAt: time.Now().UTC(),
	})
	if err != nil {
		return err
	}
	fmt.Printf("confirmed references: %d\n", len(confirmation.Confirmed.Items))

	// The production TUI entry: launcher + runtime + event bridge.
	started := time.Now()
	rt, err := tuiapp.StartWritingRuntime(workDir, "")
	if err != nil {
		return fmt.Errorf("StartWritingRuntime: %w", err)
	}

	deadline := time.After(25 * time.Minute)
	var done *writingtui.RuntimeDoneMsg
pump:
	for {
		msgCh := make(chan any, 1)
		go func() { msgCh <- rt.NextEventCmd()() }()
		select {
		case raw := <-msgCh:
			switch msg := raw.(type) {
			case nil:
				break pump // channel closed
			case writingtui.RuntimeDoneMsg:
				done = &msg
				break pump
			case writingtui.RuntimeEventMsg:
				logRuntimeEvent(writingtui.RuntimeEvent(msg))
			}
		case <-deadline:
			rt.Stop()
			return fmt.Errorf("smoke timed out after 25m")
		}
	}

	if done == nil {
		return fmt.Errorf("runtime channel closed without RuntimeDoneMsg")
	}
	if done.Error != nil {
		return fmt.Errorf("runtime done with error: %w", done.Error)
	}
	fmt.Printf("runtime done: success=%v elapsed=%s\n", done.Success, time.Since(started).Round(time.Second))

	progress, ok, err := runtimeapp.LoadProgress(workDir)
	if err != nil || !ok {
		return fmt.Errorf("load progress: ok=%v err=%v", ok, err)
	}
	fmt.Printf("progress: phase=%s status=%s completed=%v pending=%v\n",
		progress.Phase, progress.Status, progress.CompletedChapters, progress.PendingChapters)

	// Final export, same call the ExportSummary screen makes.
	input, err := finalexport.LoadInput(s)
	if err != nil {
		return err
	}
	result, err := finalexport.ExportFinal(s, input, finalexport.Options{Now: time.Now().UTC()})
	if err != nil {
		return err
	}
	fmt.Printf("export: outputs=%d docx=%v issues=%v\n", len(result.Outputs), result.DocxWritten, result.Issues)
	fmt.Printf("quality conclusion: %v\n", result.Metadata["quality_conclusion"])
	if input.Quality.Available {
		supported, other := 0, 0
		for _, node := range input.Quality.ClaimGraph.Claims {
			if node.Support == quality.SupportSupported {
				supported++
			} else {
				other++
			}
		}
		fmt.Printf("claims=%d supported=%d other=%d gate=%s\n",
			len(input.Quality.ClaimGraph.Claims), supported, other, input.Quality.GateOutcome.Conclusion)
	}
	fmt.Println("SMOKE OK")
	return nil
}

func logRuntimeEvent(ev writingtui.RuntimeEvent) {
	switch ev.Kind {
	case writingtui.EventContentDelta:
		return // too noisy for the console; the TUI renders these live
	case writingtui.EventUsageUpdate:
		return
	case writingtui.EventStepStarted, writingtui.EventStepDone, writingtui.EventStepFailed:
		fmt.Printf("[%s] %s %s %s\n", time.Now().Format("15:04:05"), ev.Kind, ev.Step, ev.Message)
	case writingtui.EventChapterStatus:
		fmt.Printf("[%s] chapter %s -> %v\n", time.Now().Format("15:04:05"), ev.ChapterID, ev.Fields["status"])
	default:
		fmt.Printf("[%s] %s %s\n", time.Now().Format("15:04:05"), ev.Kind, ev.Message)
	}
}
