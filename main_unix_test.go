//go:build unix

package main

import (
	"testing"
)

func TestRunSubcommandMapsSignalExitCode(t *testing.T) {
	_, stderr, exitCode := runBuiltCommand(t, "run", "--", "/bin/sh", "-c", "kill -TERM $$")
	if exitCode != 143 {
		t.Fatalf("unexpected signal-mapped exit code: got %d want %d\nstderr=%s", exitCode, 143, stderr)
	}
}
