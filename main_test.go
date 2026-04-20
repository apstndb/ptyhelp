package main

import (
	"context"
	"errors"
	"os"
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

func runTestCommandResult(t *testing.T, dir, name string, args ...string) ([]byte, error) {
	t.Helper()

	timeout := 2 * time.Minute
	if deadline, ok := t.Deadline(); ok {
		remaining := time.Until(deadline)
		if remaining > time.Second {
			remaining -= time.Second
		} else {
			remaining = time.Millisecond
		}
		if remaining < timeout {
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
	}
	return out, err
}

func runTestCommand(t *testing.T, dir, name string, args ...string) []byte {
	t.Helper()

	out, err := runTestCommandResult(t, dir, name, args...)
	if err != nil {
		t.Fatalf("exit error: %v\n%s", err, out)
	}
	return out
}

func buildTestBinary(t *testing.T, dir string) string {
	t.Helper()

	bin := filepath.Join(t.TempDir(), "ptyhelp")
	if runtime.GOOS == "windows" {
		bin += ".exe"
	}
	runTestCommand(t, dir, "go", "build", "-o", bin, ".")
	return bin
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

func TestHelperExit42(t *testing.T) {
	if os.Getenv("GO_WANT_HELPER_EXIT_42") != "1" {
		return
	}
	os.Exit(42)
}

func TestRunSubcommandPropagatesExitCode(t *testing.T) {
	dir := moduleDir(t)
	bin := buildTestBinary(t, dir)
	timeout := 2 * time.Minute
	if deadline, ok := t.Deadline(); ok {
		remaining := time.Until(deadline)
		if remaining > time.Second {
			remaining -= time.Second
		} else {
			remaining = time.Millisecond
		}
		if remaining < timeout {
			timeout = remaining
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, bin, "run", "--", os.Args[0], "-test.run=TestHelperExit42")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GO_WANT_HELPER_EXIT_42=1")
	out, err := cmd.CombinedOutput()
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		t.Fatalf("command timed out after %s: %v\n%s", timeout, err, out)
	}
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("expected exit error, got %v\n%s", err, out)
	}
	if got, want := exitErr.ExitCode(), 42; got != want {
		t.Fatalf("unexpected exit code: got %d want %d\n%s", got, want, out)
	}
}

func TestRunSubcommandMapsSignalExitCode(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("signals are handled differently on Windows")
	}
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
