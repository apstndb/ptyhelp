package main

import (
	"context"
	"errors"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func moduleDir(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	return filepath.Dir(file)
}

func runTestCommand(t *testing.T, dir, name string, args ...string) []byte {
	t.Helper()

	timeout := 2 * time.Minute
	if deadline, ok := t.Deadline(); ok {
		if remaining := time.Until(deadline) - time.Second; remaining < timeout {
			timeout = remaining
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir

	out, err := cmd.CombinedOutput()
	if err != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			t.Fatalf("command timed out after %s: %v\n%s", timeout, err, out)
		}
		t.Fatalf("exit error: %v\n%s", err, out)
	}
	return out
}

func TestSubcommandHelp(t *testing.T) {
	dir := moduleDir(t)
	for _, tc := range []struct {
		name string
		args []string
	}{
		{"run", []string{"run", "--help"}},
		{"run_short", []string{"run", "-h"}},
		{"patch", []string{"patch", "--help"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			out := runTestCommand(t, dir, "go", append([]string{"run", "."}, tc.args...)...)
			if len(out) == 0 {
				t.Fatal("expected usage output")
			}
		})
	}
}
