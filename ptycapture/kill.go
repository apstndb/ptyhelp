package ptycapture

import (
	"context"
	"os"
	"os/exec"
	"time"
)

func startKillWatcher(ctx context.Context, cmd *exec.Cmd, killAfter time.Duration) func() {
	if cmd.Process == nil {
		return func() {}
	}
	done := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			if killAfter > 0 {
				_ = cmd.Process.Signal(os.Interrupt)
				timer := time.NewTimer(killAfter)
				select {
				case <-timer.C:
					_ = cmd.Process.Kill()
				case <-done:
					timer.Stop()
				}
			} else {
				_ = cmd.Process.Kill()
			}
		case <-done:
		}
	}()
	return func() { close(done) }
}
