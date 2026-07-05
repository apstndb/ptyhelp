package main

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

var testBinaryPath string

func TestMain(m *testing.M) {
	os.Exit(runTestMain(m))
}

func runTestMain(m *testing.M) int {
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

	return m.Run()
}

func moduleDir(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	return filepath.Dir(file)
}

func testContext(t *testing.T) (context.Context, context.CancelFunc, time.Duration) {
	t.Helper()
	timeout := 30 * time.Second
	if deadline, ok := t.Deadline(); ok {
		if remaining := time.Until(deadline) - time.Second; remaining > 0 && remaining < timeout {
			timeout = remaining
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	return ctx, cancel, timeout
}

func runTestCommand(t *testing.T, dir, name string, args ...string) []byte {
	t.Helper()

	ctx, cancel, timeout := testContext(t)
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
	ctx, cancel, timeout := testContext(t)
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
		{"version", []string{"version"}},
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

func TestPatchFenceNoneFromStdin(t *testing.T) {
	target := filepath.Join(t.TempDir(), "README.md")
	base := "before\n<!-- T begin -->\nold\n<!-- T end -->\nafter\n"
	if err := os.WriteFile(target, []byte(base), 0o644); err != nil {
		t.Fatal(err)
	}

	cmd := exec.Command(testBinaryPath, "patch", "-file", target, "-marker", "T", "-fence=none", "-")
	cmd.Dir = moduleDir(t)
	cmd.Stdin = strings.NewReader("raw body\n")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("patch failed: %v\n%s", err, out)
	}

	got, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	want := "before\n<!-- T begin -->\nraw body\n<!-- T end -->\nafter\n"
	if string(got) != want {
		t.Fatalf("unexpected patched file:\ngot:\n%s\nwant:\n%s", got, want)
	}
}

