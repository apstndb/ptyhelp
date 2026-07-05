package ptycapture

import (
	"context"
	"os/exec"
	"time"
)

func waitForCommand(ctx context.Context, cmd *exec.Cmd, kill func()) error {
	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()

	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		if kill != nil {
			kill()
		}
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
		select {
		case err := <-done:
			return err
		case <-time.After(5 * time.Second):
			return ctx.Err()
		}
	}
}
