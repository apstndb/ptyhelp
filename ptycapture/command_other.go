//go:build !unix

package ptycapture

import (
	"os"
	"os/exec"
)

func configurePlainCommand(_ *exec.Cmd) {}

func plainCommandSignals(cmd *exec.Cmd) commandSignals {
	if cmd.Process == nil {
		return commandSignals{}
	}
	return commandSignals{
		graceful: func() error { return cmd.Process.Signal(os.Interrupt) },
		force:    cmd.Process.Kill,
	}
}
