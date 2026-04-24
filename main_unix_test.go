//go:build unix

package main

import (
	"errors"
	"os/exec"
	"testing"
)

func TestRunSubcommandMapsSignalExitCode(t *testing.T) {
	dir := moduleDir(t)
	bin := buildTestBinary(t, dir)
	out, err := runTestCommandResult(t, dir, bin, "run", "--", "/bin/sh", "-c", "kill -TERM $$")
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("expected exit error, got %v\n%s", err, out)
	}
	if got, want := exitErr.ExitCode(), 143; got != want {
		t.Fatalf("unexpected signal-mapped exit code: got %d want %d\n%s", got, want, out)
	}
}
