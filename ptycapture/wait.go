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

func waitForCommand(ctx context.Context, cmd *exec.Cmd, killAfter time.Duration, signals commandSignals) (error, bool) {
	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()

	select {
	case err := <-done:
		return err, false
	case <-ctx.Done():
		// Prefer an exit that was already observable when cancellation won the
		// outer select, whose ready cases are otherwise chosen at random.
		select {
		case err := <-done:
			return err, false
		default:
		}
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
				return err, true
			}
			<-timer.C
			if !signals.remaining() {
				return err, true
			}
			if signals.force != nil {
				_ = signals.force()
			}
			return err, true
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
		return err, true
	case <-timer.C:
		return ctx.Err(), true
	}
}
