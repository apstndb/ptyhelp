package main

import (
	"bytes"
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

const markerFileBase = "before\n<!-- T begin -->\nold\n<!-- T end -->\nafter\n"

func writeMarkerFile(t *testing.T, path string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(markerFileBase), 0o644); err != nil {
		t.Fatal(err)
	}
}

func readTestFile(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func patchVersionArgs(target, output string, extra ...string) []string {
	args := []string{"patch", "-file", target, "-marker", "T", "-o", output}
	args = append(args, extra...)
	return append(args, "--", testBinaryPath, "version")
}

func TestPatchDownstreamFlagCompositions(t *testing.T) {
	for _, tc := range []struct {
		name  string
		extra []string
		fence string
	}{
		{name: "plain_raw", extra: []string{"-fence=none"}, fence: "none"},
		{name: "pty_fenced_normalized", extra: []string{"-cols", "120", "-normalize-eol=lf"}, fence: "text"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			target := filepath.Join(dir, "README.md")
			output := filepath.Join(dir, "help.txt")
			writeMarkerFile(t, target)

			_, stderr, code := runBuiltCommand(t, patchVersionArgs(target, output, tc.extra...)...)
			if code != 0 {
				t.Fatalf("exit code = %d, want 0\nstderr=%s", code, stderr)
			}

			sidecar := readTestFile(t, output)
			if bytes.Contains(sidecar, []byte("\r\n")) && tc.fence == "text" {
				t.Fatalf("sidecar contains CRLF after normalization: %q", sidecar)
			}
			body := strings.TrimRight(string(sidecar), "\n")
			if tc.fence == "text" {
				body = "```text\n" + body + "\n```"
			}
			want := "before\n<!-- T begin -->\n" + body + "\n<!-- T end -->\nafter\n"
			if got := string(readTestFile(t, target)); got != want {
				t.Fatalf("patched target mismatch\ngot:\n%s\nwant:\n%s", got, want)
			}
		})
	}
}

func TestPatchCheckOutputFile(t *testing.T) {
	setup := func(t *testing.T) (target, output string, targetData, outputData []byte) {
		t.Helper()
		dir := t.TempDir()
		target = filepath.Join(dir, "README.md")
		output = filepath.Join(dir, "help.txt")
		writeMarkerFile(t, target)
		_, stderr, code := runBuiltCommand(t, patchVersionArgs(target, output, "-fence=none")...)
		if code != 0 {
			t.Fatalf("prepare exit code = %d, want 0\nstderr=%s", code, stderr)
		}
		return target, output, readTestFile(t, target), readTestFile(t, output)
	}

	t.Run("fresh", func(t *testing.T) {
		target, output, targetData, outputData := setup(t)
		_, stderr, code := runBuiltCommand(t, patchVersionArgs(target, output, "-check", "-fence=none")...)
		if code != 0 {
			t.Fatalf("exit code = %d, want 0\nstderr=%s", code, stderr)
		}
		if !bytes.Equal(readTestFile(t, target), targetData) || !bytes.Equal(readTestFile(t, output), outputData) {
			t.Fatal("check mode modified generated files")
		}
	})

	t.Run("stale", func(t *testing.T) {
		target, output, targetData, _ := setup(t)
		stale := []byte("stale\n")
		if err := os.WriteFile(output, stale, 0o644); err != nil {
			t.Fatal(err)
		}
		_, stderr, code := runBuiltCommand(t, patchVersionArgs(target, output, "-check", "-fence=none")...)
		if code != 1 {
			t.Fatalf("exit code = %d, want 1\nstderr=%s", code, stderr)
		}
		if !strings.Contains(string(stderr), "is stale (-o)") {
			t.Fatalf("stderr = %q, want stale -o message", stderr)
		}
		if !bytes.Equal(readTestFile(t, target), targetData) || !bytes.Equal(readTestFile(t, output), stale) {
			t.Fatal("check mode modified stale files")
		}
	})

	t.Run("missing", func(t *testing.T) {
		target, output, targetData, _ := setup(t)
		if err := os.Remove(output); err != nil {
			t.Fatal(err)
		}
		_, stderr, code := runBuiltCommand(t, patchVersionArgs(target, output, "-check", "-fence=none")...)
		if code != 1 {
			t.Fatalf("exit code = %d, want 1\nstderr=%s", code, stderr)
		}
		if !strings.Contains(string(stderr), "is stale (-o)") {
			t.Fatalf("stderr = %q, want stale -o message", stderr)
		}
		if !bytes.Equal(readTestFile(t, target), targetData) {
			t.Fatal("check mode modified target")
		}
		if _, err := os.Stat(output); !os.IsNotExist(err) {
			t.Fatalf("missing output was created: %v", err)
		}
	})
}

func TestPatchDryRunOutputFileRemainsNoWrite(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "README.md")
	output := filepath.Join(dir, "help.txt")
	writeMarkerFile(t, target)
	outputData := []byte("existing sidecar\n")
	if err := os.WriteFile(output, outputData, 0o644); err != nil {
		t.Fatal(err)
	}

	stdout, stderr, code := runBuiltCommand(t, patchVersionArgs(target, output, "-dry-run", "-fence=none")...)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0\nstderr=%s", code, stderr)
	}
	if len(stdout) == 0 {
		t.Fatal("dry-run did not print the stale target")
	}
	if got := string(readTestFile(t, target)); got != markerFileBase {
		t.Fatalf("dry-run modified target:\n%s", got)
	}
	if !bytes.Equal(readTestFile(t, output), outputData) {
		t.Fatal("dry-run modified -o output")
	}
}

func TestPatchRejectsAliasedOutput(t *testing.T) {
	for _, mode := range []string{"write", "check", "dry_run"} {
		t.Run(mode, func(t *testing.T) {
			target := filepath.Join(t.TempDir(), "README.md")
			writeMarkerFile(t, target)
			args := []string{"patch", "-file", target, "-marker", "T", "-o", target}
			switch mode {
			case "check":
				args = append(args, "-check")
			case "dry_run":
				args = append(args, "-dry-run")
			}
			args = append(args, "--", filepath.Join(t.TempDir(), "missing-command"))

			_, stderr, code := runBuiltCommand(t, args...)
			if code != 1 {
				t.Fatalf("exit code = %d, want 1\nstderr=%s", code, stderr)
			}
			if !strings.Contains(string(stderr), "-file and -o must refer to different files") {
				t.Fatalf("stderr = %q, want alias error", stderr)
			}
			if got := string(readTestFile(t, target)); got != markerFileBase {
				t.Fatalf("aliased output changed target:\n%s", got)
			}
		})
	}
}

func TestPathsReferToSameFile(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "target.md")
	other := filepath.Join(dir, "other.md")
	writeMarkerFile(t, target)
	writeMarkerFile(t, other)

	for _, tc := range []struct {
		name   string
		output string
		want   bool
	}{
		{name: "empty", output: "", want: false},
		{name: "exact", output: target, want: true},
		{name: "cleaned", output: filepath.Join(dir, ".", "target.md"), want: true},
		{name: "distinct", output: other, want: false},
		{name: "missing", output: filepath.Join(dir, "missing.md"), want: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := pathsReferToSameFile(target, tc.output)
			if err != nil {
				t.Fatal(err)
			}
			if got != tc.want {
				t.Fatalf("pathsReferToSameFile() = %v, want %v", got, tc.want)
			}
		})
	}

	hardlink := filepath.Join(dir, "hardlink.md")
	if err := os.Link(target, hardlink); err != nil {
		t.Logf("hardlink test skipped: %v", err)
	} else if got, err := pathsReferToSameFile(target, hardlink); err != nil || !got {
		t.Fatalf("hardlink alias = %v, %v; want true, nil", got, err)
	}

	symlink := filepath.Join(dir, "symlink.md")
	if err := os.Symlink(target, symlink); err != nil {
		t.Logf("symlink test skipped: %v", err)
	} else if got, err := pathsReferToSameFile(target, symlink); err != nil || !got {
		t.Fatalf("symlink alias = %v, %v; want true, nil", got, err)
	}
}
