package ptycapture

import (
	"context"
	"os"
	"os/exec"
	"sync"
	"time"
)

func startKillWatcher(ctx context.Context, cmd *exec.Cmd, killAfter time.Duration) func() {
	if cmd.Process == nil {
		return func() {}
	}
	done := make(chan struct{})
	var once sync.Once
	stop := func() { once.Do(func() { close(done) }) }
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
	return stop
}
