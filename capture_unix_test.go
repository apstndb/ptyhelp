//go:build unix

package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestRunSubcommandPTY(t *testing.T) {
	dir := moduleDir(t)
	cmd := exec.Command("go", "run", ".", "run", "-cols", "120", "--", "/bin/sh", "-c", "printf hello")
	cmd.Dir = dir

	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("exit error: %v\n%s", err, out)
	}
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

	cmd := exec.Command("go", "run", ".", "patch", "-file", target, "-marker", "T", "-cols", "120", "--", "/bin/sh", "-c", "printf hello")
	cmd.Dir = dir

	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("exit error: %v\n%s", err, out)
	}
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
