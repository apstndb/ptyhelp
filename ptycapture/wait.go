package ptycapture

import (
	"context"
	"os/exec"
	"time"
)

const forceKillWait = 5 * time.Second

type commandSignals struct {
	graceful  func() error
	force     func() error
	remaining func() bool
}

func waitForCommand(ctx context.Context, cmd *exec.Cmd, killAfter time.Duration, signals commandSignals) error {
	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()

	select {
	case err := <-done:
		return err
	case <-ctx.Done():
	}

	if killAfter > 0 {
		if signals.graceful != nil {
			_ = signals.graceful()
		}
		timer := time.NewTimer(killAfter)
		select {
		case err := <-done:
			if signals.remaining == nil || !signals.remaining() {
				timer.Stop()
				return err
			}
			<-timer.C
			if !signals.remaining() {
				return err
			}
			if signals.force != nil {
				_ = signals.force()
			}
			return err
		case <-timer.C:
		}
	}

	if signals.force != nil {
		_ = signals.force()
	}

	timer := time.NewTimer(forceKillWait)
	defer timer.Stop()
	select {
	case err := <-done:
		return err
	case <-timer.C:
		return ctx.Err()
	}
}
