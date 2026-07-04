package ptycapture

import (
	"context"
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
			_ = cmd.Process.Kill()
			if killAfter > 0 {
				timer := time.NewTimer(killAfter)
				select {
				case <-timer.C:
					_ = cmd.Process.Kill()
				case <-done:
					timer.Stop()
				}
			}
		case <-done:
		}
	}()
	return func() { close(done) }
}
