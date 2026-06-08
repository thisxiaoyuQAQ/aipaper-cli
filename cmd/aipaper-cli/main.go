package main

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/cli"
	tuiapp "github.com/thisxiaoyuQAQ/aipaper-cli/internal/tui/app"
)

func main() {
	os.Exit(run(context.Background(), os.Args[1:], os.Stdin, os.Stdout, os.Stderr, nil))
}

func run(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer, runner tuiapp.ProgramRunner) int {
	if len(args) > 0 {
		return cli.Run(ctx, args, stdout, stderr)
	}
	if err := tuiapp.RunProgram(ctx, tuiapp.ProgramOptions{
		WorkDir: ".",
		Input:   stdin,
		Output:  stdout,
		Runner:  runner,
	}); err != nil {
		fmt.Fprintf(stderr, "tui: %v\n", err)
		return 1
	}
	return 0
}
