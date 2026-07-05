//go:build unix

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunSubcommandMapsSignalExitCode(t *testing.T) {
	_, stderr, exitCode := runBuiltCommand(t, "run", "--", "/bin/sh", "-c", "kill -TERM $$")
	if exitCode != 143 {
		t.Fatalf("unexpected signal-mapped exit code: got %d want %d\nstderr=%s", exitCode, 143, stderr)
	}
}

func TestPatchSkipsFileOnChildFailure(t *testing.T) {
	target := filepath.Join(t.TempDir(), "README.md")
	base := "before\n<!-- T begin -->\nold\n<!-- T end -->\nafter\n"
	if err := os.WriteFile(target, []byte(base), 0o644); err != nil {
		t.Fatal(err)
	}
	outPath := filepath.Join(t.TempDir(), "out.txt")

	_, _, code := runBuiltCommand(t, "patch", "-file", target, "-marker", "T", "-o", outPath, "--", "/bin/sh", "-c", "echo patched; exit 42")
	if code != 42 {
		t.Fatalf("exit code = %d, want 42", code)
	}

	got, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != base {
		t.Fatalf("target file changed on failure:\ngot:\n%s\nwant:\n%s", got, base)
	}
	if _, err := os.Stat(outPath); !os.IsNotExist(err) {
		t.Fatalf("expected -o file to be skipped, stat err = %v", err)
	}
}

func TestRunSubcommandPropagatesExitCode(t *testing.T) {
	_, _, code := runBuiltCommand(t, "run", "--", "/bin/sh", "-c", "exit 7")
	if code != 7 {
		t.Fatalf("exit code = %d, want 7", code)
	}
}

func TestPatchStderrMerge(t *testing.T) {
	target := filepath.Join(t.TempDir(), "README.md")
	base := "before\n<!-- T begin -->\nold\n<!-- T end -->\nafter\n"
	if err := os.WriteFile(target, []byte(base), 0o644); err != nil {
		t.Fatal(err)
	}

	_, _, code := runBuiltCommand(t, "patch", "-file", target, "-marker", "T", "-stderr=merge", "--", "/bin/sh", "-c", "printf out; printf err 1>&2")
	if code != 0 {
		t.Fatalf("exit code = %d", code)
	}

	got, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	want := "before\n<!-- T begin -->\n```text\nouterr\n```\n<!-- T end -->\nafter\n"
	if string(got) != want {
		t.Fatalf("unexpected patched file:\ngot:\n%s\nwant:\n%s", got, want)
	}
}

func TestPatchCheckDetectsStale(t *testing.T) {
	target := filepath.Join(t.TempDir(), "README.md")
	base := "before\n<!-- T begin -->\n```text\nold\n```\n<!-- T end -->\nafter\n"
	if err := os.WriteFile(target, []byte(base), 0o644); err != nil {
		t.Fatal(err)
	}

	_, _, code := runBuiltCommand(t, "patch", "-file", target, "-marker", "T", "-check", "--", "/bin/sh", "-c", "printf new")
	if code != 1 {
		t.Fatalf("exit code = %d, want 1 for stale file", code)
	}

	_, _, code = runBuiltCommand(t, "patch", "-file", target, "-marker", "T", "-check", "--", "/bin/sh", "-c", "printf old")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0 for up-to-date file", code)
	}
}

func TestPatchDryRunWritesToStdout(t *testing.T) {
	target := filepath.Join(t.TempDir(), "README.md")
	base := "before\n<!-- T begin -->\n```text\nold\n```\n<!-- T end -->\nafter\n"
	if err := os.WriteFile(target, []byte(base), 0o644); err != nil {
		t.Fatal(err)
	}

	stdout, _, code := runBuiltCommand(t, "patch", "-file", target, "-marker", "T", "-dry-run", "--", "/bin/sh", "-c", "printf new")
	if code != 0 {
		t.Fatalf("exit code = %d", code)
	}
	if !strings.Contains(string(stdout), "new") {
		t.Fatalf("dry-run stdout missing new content: %q", stdout)
	}

	got, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != base {
		t.Fatal("dry-run must not modify target file")
	}
}
