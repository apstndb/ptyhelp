package main

import (
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

func moduleDir(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	return filepath.Dir(file)
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
			cmd := exec.Command("go", append([]string{"run", "."}, tc.args...)...)
			cmd.Dir = dir
			out, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("exit error: %v\n%s", err, out)
			}
			if len(out) == 0 {
				t.Fatal("expected usage output")
			}
		})
	}
}
