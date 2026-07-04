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

var testBinaryPath string

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "ptyhelp-test-*")
	if err != nil {
		panic(err)
	}
	defer func() { _ = os.RemoveAll(dir) }()

	binaryName := "ptyhelp"
	if runtime.GOOS == "windows" {
		binaryName += ".exe"
	}
	testBinaryPath = filepath.Join(dir, binaryName)
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		panic("runtime.Caller failed")
	}
	cmd := exec.Command("go", "build", "-o", testBinaryPath, ".")
	cmd.Dir = filepath.Dir(file)
	if out, err := cmd.CombinedOutput(); err != nil {
		panic("build test binary: " + err.Error() + "\n" + string(out))
	}

	os.Exit(m.Run())
}

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

	timeout := 30 * time.Second
	if deadline, ok := t.Deadline(); ok {
		if remaining := time.Until(deadline) - time.Second; remaining > 0 && remaining < timeout {
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

func runBuiltCommand(t *testing.T, args ...string) (stdout, stderr []byte, exitCode int) {
	t.Helper()
	timeout := 30 * time.Second
	if deadline, ok := t.Deadline(); ok {
		if remaining := time.Until(deadline) - time.Second; remaining > 0 && remaining < timeout {
			timeout = remaining
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, testBinaryPath, args...)
	cmd.Dir = moduleDir(t)
	var outBuf, errBuf []byte
	var err error
	outBuf, err = cmd.Output()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			errBuf = exitErr.Stderr
			return outBuf, errBuf, exitErr.ExitCode()
		}
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			t.Fatalf("command timed out after %s: %v", timeout, err)
		}
		t.Fatalf("command failed: %v\nstdout=%q\nstderr=%q", err, outBuf, errBuf)
	}
	return outBuf, errBuf, 0
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
			out := runTestCommand(t, dir, testBinaryPath, tc.args...)
			if len(out) == 0 {
				t.Fatal("expected usage output")
			}
		})
	}
}

func TestRunSubcommandForwardsChildHelp(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("PTY child forwarding is too slow under -race on Windows CI")
	}
	out, _, code := runBuiltCommand(t, "run", "--", "go", "version")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0\nstderr=%q", code, out)
	}
	if len(out) == 0 {
		t.Fatal("expected child help on stdout")
	}
}
