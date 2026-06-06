package store

import (
	"path/filepath"

	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/contracts"
)

type Store struct {
	root string
}

func New(workDir string) Store {
	if workDir == "" {
		workDir = "."
	}
	return Store{root: filepath.Join(workDir, contracts.DefaultOutputDir, contracts.DefaultProject)}
}

func NewAt(root string) Store {
	return Store{root: root}
}

func (s Store) Root() string {
	return s.root
}

func (s Store) Path(parts ...string) string {
	all := append([]string{s.root}, parts...)
	return filepath.Join(all...)
}

func (s Store) Rel(parts ...string) string {
	return filepath.ToSlash(filepath.Join(parts...))
}

func (s Store) RunPath() string {
	return s.Path("run.json")
}

func (s Store) ProgressPath() string {
	return s.Path("progress.json")
}

func (s Store) RequirementsPath() string {
	return s.Path("requirements.json")
}

func (s Store) CheckpointsDir() string {
	return s.Path("checkpoints")
}

func (s Store) StepCheckpointPath(step int) string {
	return s.Path("checkpoints", StepCheckpointName(step))
}

func (s Store) LatestCheckpointPath() string {
	return s.Path("checkpoints", "latest.json")
}

func StepCheckpointName(step int) string {
	return "step-" + leftPadInt(step, 6) + ".json"
}

func RequiredDirs() []string {
	return []string{
		"checkpoints",
		filepath.Join("materials", "extracted"),
		filepath.Join("materials", "parsed"),
		"references",
		"outline",
		"drafts",
		"reviews",
		"final",
	}
}

func leftPadInt(n, width int) string {
	if n < 0 {
		n = 0
	}
	digits := []byte("00000000000000000000")
	s := []byte{}
	if n == 0 {
		s = append(s, '0')
	}
	for n > 0 {
		s = append([]byte{byte('0' + n%10)}, s...)
		n /= 10
	}
	if len(s) >= width {
		return string(s)
	}
	return string(digits[:width-len(s)]) + string(s)
}
