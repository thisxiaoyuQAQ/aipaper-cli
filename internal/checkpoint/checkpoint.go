package checkpoint

import (
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/contracts"
	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/store"
)

type Checkpoint struct {
	Step         int              `json:"step"`
	Phase        string           `json:"phase"`
	Status       string           `json:"status"`
	CreatedAt    time.Time        `json:"created_at"`
	Input        map[string]any   `json:"input,omitempty"`
	Outputs      []OutputArtifact `json:"outputs"`
	StatePatch   map[string]any   `json:"state_patch,omitempty"`
	NextExpected string           `json:"next_expected,omitempty"`
}

type OutputArtifact struct {
	Kind   string `json:"kind"`
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
}

type Validation struct {
	OK      bool     `json:"ok"`
	Step    int      `json:"step,omitempty"`
	Phase   string   `json:"phase,omitempty"`
	Next    string   `json:"next_expected,omitempty"`
	Errors  []string `json:"errors,omitempty"`
	Checked []string `json:"checked,omitempty"`
}

func Record(s store.Store, cp Checkpoint, progress contracts.Progress) error {
	if cp.Step <= 0 {
		return errors.New("checkpoint step must be positive")
	}
	if cp.Phase == "" {
		return errors.New("checkpoint phase is required")
	}
	if cp.Status == "" {
		cp.Status = "success"
	}
	if cp.CreatedAt.IsZero() {
		cp.CreatedAt = time.Now().UTC()
	}
	if progress.UpdatedAt.IsZero() {
		progress.UpdatedAt = cp.CreatedAt
	}
	progress.CurrentStep = cp.Step
	if progress.Phase == "" {
		progress.Phase = cp.Phase
	}

	if err := store.EnsureLayout(s); err != nil {
		return err
	}
	if _, err := store.WriteJSON(s.StepCheckpointPath(cp.Step), cp, store.CreateOnly); err != nil {
		if strings.Contains(err.Error(), "artifact conflict") {
			return fmt.Errorf("STORE_CHECKPOINT_CONFLICT: %w", err)
		}
		return err
	}
	if _, err := store.WriteJSON(s.LatestCheckpointPath(), cp, store.Overwrite); err != nil {
		return err
	}
	if _, err := store.WriteJSON(s.ProgressPath(), progress, store.Overwrite); err != nil {
		return err
	}
	return nil
}

func LoadLatest(s store.Store) (Checkpoint, bool, error) {
	var cp Checkpoint
	err := store.ReadJSON(s.LatestCheckpointPath(), &cp)
	if errors.Is(err, os.ErrNotExist) {
		return Checkpoint{}, false, nil
	}
	if err != nil {
		return Checkpoint{}, false, err
	}
	return cp, true, nil
}

func ValidateLatest(s store.Store) Validation {
	cp, ok, err := LoadLatest(s)
	if err != nil {
		return Validation{OK: false, Errors: []string{err.Error()}}
	}
	if !ok {
		return Validation{OK: false, Errors: []string{"latest checkpoint does not exist"}}
	}
	v := Validation{
		OK:    true,
		Step:  cp.Step,
		Phase: cp.Phase,
		Next:  cp.NextExpected,
	}
	for _, out := range cp.Outputs {
		outputPath, err := cleanOutputPath(out.Path)
		if err != nil {
			v.OK = false
			v.Errors = append(v.Errors, err.Error())
			continue
		}
		diskPath := filepath.Clean(s.Path(filepath.FromSlash(outputPath)))
		if !isWithin(diskPath, s.Root()) {
			v.OK = false
			v.Errors = append(v.Errors, fmt.Sprintf("output path escapes store root: %s", out.Path))
			continue
		}
		sum, err := store.FileSHA256(diskPath)
		if err != nil {
			v.OK = false
			v.Errors = append(v.Errors, fmt.Sprintf("read %s: %v", out.Path, err))
			continue
		}
		v.Checked = append(v.Checked, outputPath)
		if sum != out.SHA256 {
			v.OK = false
			v.Errors = append(v.Errors, fmt.Sprintf("hash mismatch for %s", out.Path))
		}
	}
	return v
}

func cleanOutputPath(outputPath string) (string, error) {
	if outputPath == "" {
		return "", errors.New("output path is required")
	}
	if strings.Contains(outputPath, "\\") {
		return "", fmt.Errorf("output path must use forward slashes: %s", outputPath)
	}
	if path.IsAbs(outputPath) || filepath.IsAbs(outputPath) || filepath.VolumeName(outputPath) != "" || hasWindowsVolume(outputPath) {
		return "", fmt.Errorf("output path must be relative: %s", outputPath)
	}
	cleaned := path.Clean(outputPath)
	if cleaned == "." || cleaned == ".." || strings.HasPrefix(cleaned, "../") {
		return "", fmt.Errorf("output path escapes store root: %s", outputPath)
	}
	return cleaned, nil
}

func hasWindowsVolume(outputPath string) bool {
	if len(outputPath) < 2 || outputPath[1] != ':' {
		return false
	}
	letter := outputPath[0]
	return letter >= 'A' && letter <= 'Z' || letter >= 'a' && letter <= 'z'
}

func isWithin(path, root string) bool {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return false
	}
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return false
	}
	rel, err := filepath.Rel(absRoot, absPath)
	if err != nil {
		return false
	}
	return rel == "." || (rel != "" && rel != ".." && !startsWithDotDot(rel))
}

func startsWithDotDot(path string) bool {
	return path == ".." || len(path) > 3 && path[:3] == ".."+string(filepath.Separator)
}
