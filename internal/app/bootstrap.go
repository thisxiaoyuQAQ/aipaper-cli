package app

import (
	"errors"
	"os"
	"time"

	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/config"
	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/contracts"
	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/store"
)

type InitResult struct {
	StoreRoot string
	Created   []string
	Existing  []string
}

func Bootstrap(workDir string, cfg config.Config) (InitResult, error) {
	s := store.New(workDir)
	result := InitResult{StoreRoot: s.Root()}
	if err := store.EnsureLayout(s); err != nil {
		return result, err
	}

	now := time.Now().UTC()
	run := contracts.Run{
		RunID:        "run-" + now.Format("20060102T150405.000000000Z"),
		CreatedAt:    now,
		Provider:     cfg.Provider,
		Model:        cfg.Model,
		CostEstimate: map[string]any{},
		Events:       []contracts.RunEvent{},
	}
	progress := contracts.Progress{
		Phase:             "initialized",
		Status:            "idle",
		UpdatedAt:         run.CreatedAt,
		CompletedChapters: []string{},
		PendingChapters:   []string{},
	}
	if err := writeInitialJSON(s.RunPath(), run, &contracts.Run{}, &result); err != nil {
		return result, err
	}
	if err := writeInitialJSON(s.ProgressPath(), progress, &contracts.Progress{}, &result); err != nil {
		return result, err
	}
	return result, nil
}

func LoadProgress(workDir string) (contracts.Progress, bool, error) {
	var progress contracts.Progress
	err := store.ReadJSON(store.New(workDir).ProgressPath(), &progress)
	if errors.Is(err, os.ErrNotExist) {
		return contracts.Progress{}, false, nil
	}
	if err != nil {
		return contracts.Progress{}, false, err
	}
	return progress, true, nil
}

func writeInitialJSON(path string, value any, existing any, result *InitResult) error {
	err := store.ReadJSON(path, existing)
	if err == nil {
		result.Existing = append(result.Existing, path)
		return nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	res, err := store.WriteJSON(path, value, store.CreateOnly)
	if err != nil {
		return err
	}
	if res.AlreadyExists {
		result.Existing = append(result.Existing, path)
	} else {
		result.Created = append(result.Created, path)
	}
	return nil
}
