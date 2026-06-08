package main

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	tuiapp "github.com/thisxiaoyuQAQ/aipaper-cli/internal/tui/app"
)

type fakeProgramRunner struct {
	called int
	model  tuiapp.RootModel
	opts   tuiapp.ProgramOptions
	err    error
}

func (r *fakeProgramRunner) Run(ctx context.Context, model tuiapp.RootModel, opts tuiapp.ProgramOptions) error {
	_ = ctx
	r.called++
	r.model = model
	r.opts = opts
	return r.err
}

func TestRunWithoutArgsCallsTUIRunner(t *testing.T) {
	var stdin, stdout, stderr bytes.Buffer
	runner := &fakeProgramRunner{}

	code := run(context.Background(), nil, &stdin, &stdout, &stderr, runner)
	if code != 0 {
		t.Fatalf("code = %d, stderr = %s", code, stderr.String())
	}
	if runner.called != 1 {
		t.Fatalf("runner called %d times, want 1", runner.called)
	}
	if runner.model.CurrentScreen != tuiapp.ScreenConfigWizard {
		t.Fatalf("initial screen = %q, want %q", runner.model.CurrentScreen, tuiapp.ScreenConfigWizard)
	}
	if runner.opts.WorkDir != "." {
		t.Fatalf("WorkDir = %q, want .", runner.opts.WorkDir)
	}
}

func TestRunWithArgsUsesCLI(t *testing.T) {
	var stdout, stderr bytes.Buffer
	runner := &fakeProgramRunner{}

	code := run(context.Background(), []string{"help"}, nil, &stdout, &stderr, runner)
	if code != 0 {
		t.Fatalf("code = %d, stderr = %s", code, stderr.String())
	}
	if runner.called != 0 {
		t.Fatalf("runner called %d times, want 0", runner.called)
	}
	if !strings.Contains(stdout.String(), "aipaper-cli") {
		t.Fatalf("stdout = %s, want help text", stdout.String())
	}
}

func TestRunReturnsNonZeroWhenTUIFails(t *testing.T) {
	var stdout, stderr bytes.Buffer
	runner := &fakeProgramRunner{err: errors.New("boom")}

	code := run(context.Background(), nil, nil, &stdout, &stderr, runner)
	if code == 0 {
		t.Fatalf("code = 0, want non-zero")
	}
	if !strings.Contains(stderr.String(), "tui: boom") {
		t.Fatalf("stderr = %s, want TUI error", stderr.String())
	}
}
