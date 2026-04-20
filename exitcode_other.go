//go:build !unix

package main

import "os/exec"

func exitCodeFromExitError(exitErr *exec.ExitError) int {
	if exitCode := exitErr.ExitCode(); exitCode >= 0 {
		return exitCode
	}
	return 1
}
