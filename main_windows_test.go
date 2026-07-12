//go:build windows

package main

import (
	"bytes"
	"testing"
	"time"
)

func TestRunSubcommandWindowsDrainsConPTYOutput(t *testing.T) {
	const size = 256 << 10
	const endMarker = "PTYHELP-END"
	stdout, stderr, code := runBuiltCommand(
		t,
		"run",
		"--",
		"powershell.exe",
		"-NoProfile",
		"-NonInteractive",
		"-Command",
		"[Console]::Out.Write(('x' * 262144) + ([char[]](80,84,89,72,69,76,80,45,69,78,68) -join ''))",
	)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0\nstderr=%q", code, stderr)
	}
	if got := bytes.Count(stdout, []byte{'x'}); got < size {
		t.Fatalf("captured %d data bytes, want at least %d (total output %d bytes)", got, size, len(stdout))
	}
	if !bytes.Contains(stdout, []byte(endMarker)) {
		t.Fatalf("captured output is missing final marker %q", endMarker)
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
