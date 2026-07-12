//go:build windows

package main

import (
	"bytes"
	"testing"
	"time"
)

func TestRunSubcommandWindowsDrainsConPTYOutput(t *testing.T) {
	const size = 256 << 10
	stdout, stderr, code := runBuiltCommand(
		t,
		"run",
		"--",
		"powershell.exe",
		"-NoProfile",
		"-NonInteractive",
		"-Command",
		"[Console]::Out.Write('x' * 262144)",
	)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0\nstderr=%q", code, stderr)
	}
	if got := bytes.Count(stdout, []byte{'x'}); got != size {
		t.Fatalf("captured %d data bytes, want %d (total output %d bytes)", got, size, len(stdout))
	}
}

func TestRunSubcommandWindowsSendsTerminalEOF(t *testing.T) {
	_, stderr, code := runBuiltCommand(
		t,
		"run",
		"--",
		"powershell.exe",
		"-NoProfile",
		"-NonInteractive",
		"-Command",
		"[Console]::In.ReadToEnd() | Out-Null",
	)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0\nstderr=%q", code, stderr)
	}
}

func TestRunSubcommandWindowsHonorsKillAfter(t *testing.T) {
	const killAfter = 400 * time.Millisecond
	started := time.Now()
	_, _, code := runBuiltCommand(
		t,
		"run",
		"-timeout=100ms",
		"-kill-after=400ms",
		"--",
		"powershell.exe",
		"-NoProfile",
		"-NonInteractive",
		"-Command",
		"Start-Sleep -Seconds 10",
	)
	elapsed := time.Since(started)
	if code == 0 {
		t.Fatal("expected timeout failure")
	}
	if elapsed < killAfter {
		t.Fatalf("command returned after %s, before kill-after grace %s", elapsed, killAfter)
	}
}
