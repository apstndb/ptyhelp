//go:build unix

package main

import (
	"os"
	"path/filepath"
	"testing"
)

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
