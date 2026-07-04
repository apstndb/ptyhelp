//go:build unix

package main

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/creack/pty"
	"golang.org/x/term"
)

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

func TestRunSubcommandPTY(t *testing.T) {
	out, _, code := runBuiltCommand(t, "run", "-cols", "120", "--", "/bin/sh", "-c", "printf hello")
	if code != 0 {
		t.Fatalf("exit code = %d", code)
	}
	if got, want := string(out), "hello"; got != want {
		t.Fatalf("unexpected output: got %q want %q", got, want)
	}
}

func TestPatchSubcommandPTY(t *testing.T) {
	target := filepath.Join(t.TempDir(), "README.md")
	base := "before\n<!-- T begin -->\nold\n<!-- T end -->\nafter\n"
	if err := os.WriteFile(target, []byte(base), 0o644); err != nil {
		t.Fatal(err)
	}

	_, _, code := runBuiltCommand(t, "patch", "-file", target, "-marker", "T", "-cols", "120", "--", "/bin/sh", "-c", "printf hello")
	if code != 0 {
		t.Fatalf("exit code = %d", code)
	}

	got, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	want := "before\n<!-- T begin -->\n```text\nhello\n```\n<!-- T end -->\nafter\n"
	if string(got) != want {
		t.Fatalf("unexpected patched file:\ngot:\n%s\nwant:\n%s", got, want)
	}
}

func TestRunSubcommandPTYEOFOnEmptyStdin(t *testing.T) {
	stdinMaster, stdinSlave, err := pty.Open()
	if err != nil {
		t.Fatal(err)
	}
	defer stdinMaster.Close()
	defer stdinSlave.Close()

	ctx, cancel, timeout := testContext(t)
	defer cancel()

	cmd := exec.CommandContext(ctx, testBinaryPath, "run", "-cols", "120", "--", "/bin/sh", "-c", "cat >/dev/null; printf done")
	cmd.Dir = moduleDir(t)
	cmd.Stdin = stdinSlave
	out, err := cmd.CombinedOutput()
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		t.Fatalf("command timed out after %s: %v\n%s", timeout, err, out)
	}
	if err != nil {
		t.Fatalf("unexpected command error: %v\n%s", err, out)
	}
	if got, want := string(out), "done"; got != want {
		t.Fatalf("unexpected output: got %q want %q", got, want)
	}
}

func TestRunSubcommandPTYPreservesTTYOnStdin(t *testing.T) {
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		t.Skip("parent stdin is not a terminal in this test environment")
	}
	out, _, code := runBuiltCommand(t, "run", "-cols", "120", "--", "/bin/sh", "-c", "if [ -t 0 ]; then printf tty0; else printf notty0; fi")
	if code != 0 {
		t.Fatalf("exit code = %d", code)
	}
	if got, want := string(out), "tty0"; got != want {
		t.Fatalf("unexpected output: got %q want %q", got, want)
	}
}

func TestPatchSubcommandPlain(t *testing.T) {
	target := filepath.Join(t.TempDir(), "README.md")
	base := "before\n<!-- T begin -->\nold\n<!-- T end -->\nafter\n"
	if err := os.WriteFile(target, []byte(base), 0o644); err != nil {
		t.Fatal(err)
	}

	_, _, code := runBuiltCommand(t, "patch", "-file", target, "-marker", "T", "--", "/bin/sh", "-c", "printf hello")
	if code != 0 {
		t.Fatalf("exit code = %d", code)
	}

	got, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	want := "before\n<!-- T begin -->\n```text\nhello\n```\n<!-- T end -->\nafter\n"
	if string(got) != want {
		t.Fatalf("unexpected patched file:\ngot:\n%s\nwant:\n%s", got, want)
	}
}

func TestRunSubcommandPTYDoesNotHangWithDaemonizedDescendant(t *testing.T) {
	out, _, code := runBuiltCommand(t, "run", "-cols", "120", "--", "/bin/sh", "-c", "(trap '' HUP; sleep 30) & printf hello")
	if code != 0 {
		t.Fatalf("exit code = %d", code)
	}
	if got, want := string(out), "hello"; got != want {
		t.Fatalf("unexpected output: got %q want %q", got, want)
	}
}

func TestPatchSubcommandDoesNotRewriteFileOnNonZeroExit(t *testing.T) {
	target := filepath.Join(t.TempDir(), "README.md")
	base := "before\n<!-- T begin -->\nold\n<!-- T end -->\nafter\n"
	if err := os.WriteFile(target, []byte(base), 0o644); err != nil {
		t.Fatal(err)
	}

	ctx, cancel, timeout := testContext(t)
	defer cancel()

	cmd := exec.CommandContext(ctx, testBinaryPath, "patch", "-file", target, "-marker", "T", "--", "/bin/sh", "-c", "printf broken; exit 42")
	cmd.Dir = moduleDir(t)
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

	got, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != base {
		t.Fatalf("unexpected patched file after non-zero exit:\ngot:\n%s\nwant:\n%s", got, base)
	}
}

func TestRunSubcommandPTYPipedStdinPreservesBytes(t *testing.T) {
	ctx, cancel, timeout := testContext(t)
	defer cancel()

	cmd := exec.CommandContext(ctx, testBinaryPath, "run", "-cols", "120", "--", "od", "-An", "-tx1", "-v")
	cmd.Dir = moduleDir(t)
	cmd.Stdin = strings.NewReader("abc")
	out, err := cmd.CombinedOutput()
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		t.Fatalf("command timed out after %s: %v\n%s", timeout, err, out)
	}
	if err != nil {
		t.Fatalf("unexpected command error: %v\n%s", err, out)
	}
	fields := strings.Fields(string(out))
	if len(fields) < 3 || fields[0] != "61" || fields[1] != "62" || fields[2] != "63" {
		t.Fatalf("missing piped stdin bytes in output: %q", out)
	}
	for _, field := range fields {
		if field == "04" {
			t.Fatalf("unexpected EOT byte in piped stdin output: %q", out)
		}
	}
}

func TestRunSubcommandPTYDevNullStdinDoesNotInjectEOT(t *testing.T) {
	devNull, err := os.Open(os.DevNull)
	if err != nil {
		t.Fatal(err)
	}
	defer devNull.Close()

	ctx, cancel, timeout := testContext(t)
	defer cancel()

	cmd := exec.CommandContext(ctx, testBinaryPath, "run", "-cols", "120", "--", "od", "-An", "-tx1", "-v")
	cmd.Dir = moduleDir(t)
	cmd.Stdin = devNull
	out, err := cmd.CombinedOutput()
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		t.Fatalf("command timed out after %s: %v\n%s", timeout, err, out)
	}
	if err != nil {
		t.Fatalf("unexpected command error: %v\n%s", err, out)
	}
	if len(strings.Fields(string(out))) != 0 {
		t.Fatalf("unexpected bytes from /dev/null stdin: %q", out)
	}
}
