//go:build unix

package ptycapture

import (
	"os/exec"
	"syscall"
)

func configurePlainCommand(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

func plainCommandSignals(cmd *exec.Cmd) commandSignals {
	return unixCommandSignals(cmd)
}

func unixCommandSignals(cmd *exec.Cmd) commandSignals {
	if cmd.Process == nil {
		return commandSignals{}
	}
	pid := cmd.Process.Pid
	pgid, err := syscall.Getpgid(pid)
	if err != nil || !safeProcessGroup(pgid) {
		return commandSignals{
			graceful: func() error { return cmd.Process.Signal(syscall.SIGTERM) },
			force:    cmd.Process.Kill,
		}
	}
	signalGroup := func(sig syscall.Signal) error {
		return syscall.Kill(-pgid, sig)
	}
	return commandSignals{
		graceful:  func() error { return signalGroup(syscall.SIGTERM) },
		force:     func() error { return signalGroup(syscall.SIGKILL) },
		remaining: func() bool { return syscall.Kill(-pgid, 0) == nil },
	}
}

func safeProcessGroup(pgid int) bool {
	if pgid <= 1 {
		return false
	}
	selfPGID, err := syscall.Getpgid(0)
	return err == nil && pgid != selfPGID
}
