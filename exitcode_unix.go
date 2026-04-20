//go:build unix

package main

import (
	"os/exec"
	"syscall"
)

func exitCodeFromExitError(exitErr *exec.ExitError) int {
	if exitCode := exitErr.ExitCode(); exitCode >= 0 {
		return exitCode
	}
	if status, ok := exitErr.Sys().(syscall.WaitStatus); ok && status.Signaled() {
		return 128 + int(status.Signal())
	}
	return 1
}
