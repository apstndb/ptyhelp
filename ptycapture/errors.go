package ptycapture

import (
	"errors"
	"fmt"
	"os/exec"
)

// exitStatusError is returned when a child exits with a non-zero status.
type exitStatusError struct {
	code int
}

func (e *exitStatusError) Error() string {
	return fmt.Sprintf("exit status %d", e.code)
}

func (e *exitStatusError) ExitCode() int {
	return e.code
}

// ExitCode returns the subprocess exit code when err represents a normal exit failure.
func ExitCode(err error) (int, bool) {
	if err == nil {
		return 0, true
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return exitErr.ExitCode(), true
	}
	var statusErr *exitStatusError
	if errors.As(err, &statusErr) {
		return statusErr.ExitCode(), true
	}
	return 0, false
}
