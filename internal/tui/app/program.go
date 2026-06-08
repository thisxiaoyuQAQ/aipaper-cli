package app

import (
	"context"
	"io"

	tea "github.com/charmbracelet/bubbletea"
)

type ProgramOptions struct {
	WorkDir       string
	Input         io.Reader
	Output        io.Writer
	InitialScreen Screen
	Runner        ProgramRunner
}

type ProgramRunner interface {
	Run(context.Context, RootModel, ProgramOptions) error
}

type ProgramRunnerFunc func(context.Context, RootModel, ProgramOptions) error

func (f ProgramRunnerFunc) Run(ctx context.Context, model RootModel, opts ProgramOptions) error {
	return f(ctx, model, opts)
}

type BubbleTeaRunner struct{}

func RunProgram(ctx context.Context, opts ProgramOptions) error {
	runner := opts.Runner
	if runner == nil {
		runner = BubbleTeaRunner{}
	}
	model := NewRootModel(RootOptions{
		WorkDir:       opts.WorkDir,
		InitialScreen: opts.InitialScreen,
	})
	return runner.Run(ctx, model, opts)
}

func (BubbleTeaRunner) Run(ctx context.Context, model RootModel, opts ProgramOptions) error {
	programOpts := []tea.ProgramOption{tea.WithContext(ctx)}
	if opts.Input != nil {
		programOpts = append(programOpts, tea.WithInput(opts.Input))
	}
	if opts.Output != nil {
		programOpts = append(programOpts, tea.WithOutput(opts.Output))
	}
	_, err := tea.NewProgram(model, programOpts...).Run()
	return err
}
