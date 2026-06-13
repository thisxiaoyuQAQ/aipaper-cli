package app

// runtime_diag.go: optional, injectable structured logger for the writing
// runtime startup chain. The launcher records key nodes (start, prompt-sent,
// per-round idle, abort, done/error) so a real-run hang can be located after
// the fact from output/aipaper/runtime.log instead of staring at a static TUI.
//
// It only logs startup-chain nodes — never per-streaming deltas — to keep the
// log readable. On any write failure it falls back to stderr and never blocks
// the agent loop.

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/store"
)

// diagLogger records structured startup-chain lines. The zero value is a no-op
// logger, so callers that don't care (e.g. some unit paths) pay nothing.
type diagLogger struct {
	mu  sync.Mutex
	w   io.Writer
	now func() time.Time
}

// newFileDiagLogger opens (creating parents) output/aipaper/runtime.log under
// the given workDir. If the file cannot be opened it falls back to stderr.
func newFileDiagLogger(workDir string) *diagLogger {
	if workDir == "" {
		workDir = "."
	}
	root := store.New(workDir).Root()
	path := filepath.Join(root, "runtime.log")
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	w := io.Writer(f)
	if err != nil {
		w = os.Stderr // fall back; never fail the run over logging
	}
	return &diagLogger{w: w, now: func() time.Time { return time.Now().UTC() }}
}

// newBufferDiagLogger is for tests.
func newBufferDiagLogger(buf io.Writer) *diagLogger {
	return &diagLogger{w: buf, now: func() time.Time { return time.Now().UTC() }}
}

// logf writes one line prefixed with an RFC3339 timestamp. Safe for concurrent
// use; a nil receiver is a no-op.
func (d *diagLogger) logf(format string, args ...any) {
	if d == nil {
		return
	}
	ts := time.Now().UTC()
	if d.now != nil {
		ts = d.now()
	}
	line := fmt.Sprintf("[%s] %s\n", ts.Format(time.RFC3339), fmt.Sprintf(format, args...))
	d.mu.Lock()
	defer d.mu.Unlock()
	_, _ = io.WriteString(d.w, line)
}

// asStdLog exposes the sink as a *log.Logger for code paths that already take
// one (none today, but keeps the door open without a wider refactor).
func (d *diagLogger) asStdLog() *log.Logger {
	if d == nil {
		return log.New(io.Discard, "", 0)
	}
	return log.New(d.w, "", 0)
}
