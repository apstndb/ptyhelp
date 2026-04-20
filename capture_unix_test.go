//go:build unix

package main

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunSubcommandPTY(t *testing.T) {
	dir := moduleDir(t)
	out := runTestCommand(t, dir, "go", "run", ".", "run", "-cols", "120", "--", "/bin/sh", "-c", "printf hello")
	if got, want := string(out), "hello"; got != want {
		t.Fatalf("unexpected output: got %q want %q", got, want)
	}
}

func TestPatchSubcommandPTY(t *testing.T) {
	dir := moduleDir(t)
	target := filepath.Join(t.TempDir(), "README.md")
	base := "before\n<!-- T begin -->\nold\n<!-- T end -->\nafter\n"
	if err := os.WriteFile(target, []byte(base), 0o644); err != nil {
		t.Fatal(err)
	}

	out := runTestCommand(t, dir, "go", "run", ".", "patch", "-file", target, "-marker", "T", "-cols", "120", "--", "/bin/sh", "-c", "printf hello")
	if len(out) != 0 {
		t.Fatalf("unexpected command output: %q", out)
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
	dir := moduleDir(t)
	out := runTestCommand(t, dir, "go", "run", ".", "run", "-cols", "120", "--", "/bin/sh", "-c", "cat >/dev/null; printf done")
	if got, want := string(out), "done"; got != want {
		t.Fatalf("unexpected output: got %q want %q", got, want)
	}
}

func TestRunSubcommandPTYPreservesTTYOnStdin(t *testing.T) {
	dir := moduleDir(t)
	out := runTestCommand(t, dir, "go", "run", ".", "run", "-cols", "120", "--", "/bin/sh", "-c", "if [ -t 0 ]; then printf tty0; else printf notty0; fi")
	if got, want := string(out), "tty0"; got != want {
		t.Fatalf("unexpected output: got %q want %q", got, want)
	}
}

func TestPatchSubcommandPlain(t *testing.T) {
	dir := moduleDir(t)
	target := filepath.Join(t.TempDir(), "README.md")
	base := "before\n<!-- T begin -->\nold\n<!-- T end -->\nafter\n"
	if err := os.WriteFile(target, []byte(base), 0o644); err != nil {
		t.Fatal(err)
	}

	out := runTestCommand(t, dir, "go", "run", ".", "patch", "-file", target, "-marker", "T", "--", "/bin/sh", "-c", "printf hello")
	if len(out) != 0 {
		t.Fatalf("unexpected command output: %q", out)
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
	dir := moduleDir(t)
	out := runTestCommand(t, dir, "go", "run", ".", "run", "-cols", "120", "--", "/bin/sh", "-c", "(trap '' HUP; sleep 30) & printf hello")
	if got, want := string(out), "hello"; got != want {
		t.Fatalf("unexpected output: got %q want %q", got, want)
	}
}

func TestPatchSubcommandDoesNotRewriteFileOnNonZeroExit(t *testing.T) {
	dir := moduleDir(t)
	bin := buildTestBinary(t, dir)
	target := filepath.Join(t.TempDir(), "README.md")
	base := "before\n<!-- T begin -->\nold\n<!-- T end -->\nafter\n"
	if err := os.WriteFile(target, []byte(base), 0o644); err != nil {
		t.Fatal(err)
	}

	cmd := exec.Command(bin, "patch", "-file", target, "-marker", "T", "--", "/bin/sh", "-c", "printf broken; exit 42")
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
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
	dir := moduleDir(t)
	bin := buildTestBinary(t, dir)
	cmd := exec.Command(bin, "run", "-cols", "120", "--", "od", "-An", "-tx1", "-v")
	cmd.Dir = dir
	cmd.Stdin = strings.NewReader("abc")
	out, err := cmd.CombinedOutput()
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
