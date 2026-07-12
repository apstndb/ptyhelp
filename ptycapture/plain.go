package ptycapture

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"sync"
	"time"
)

const plainDrainTimeout = 100 * time.Millisecond

// CapturePlain runs argv with ordinary pipe I/O (no pseudo-terminal).
// A nil context is treated as context.Background.
func CapturePlain(ctx context.Context, opts Options, argv []string) (stdout, stderr []byte, err error) {
	if len(argv) == 0 {
		return nil, nil, fmt.Errorf("empty command")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	parentCtx := ctx
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	cmd.Cancel = func() error { return nil }
	cmd.Stdin = os.Stdin
	configurePlainCommand(cmd)

	stdoutR, stdoutW, err := os.Pipe()
	if err != nil {
		return nil, nil, err
	}
	stderrR, stderrW, err := os.Pipe()
	if err != nil {
		_ = stdoutR.Close()
		_ = stdoutW.Close()
		return nil, nil, err
	}
	cmd.Stdout = stdoutW
	cmd.Stderr = stderrW

	if err := cmd.Start(); err != nil {
		_ = stdoutR.Close()
		_ = stdoutW.Close()
		_ = stderrR.Close()
		_ = stderrW.Close()
		return nil, nil, err
	}
	_ = stdoutW.Close()
	_ = stderrW.Close()

	var wg sync.WaitGroup
	var outBuf, errBuf bytes.Buffer
	var outErr, errErr error
	wg.Add(2)
	go func() {
		defer wg.Done()
		outErr = copyLimited(&outBuf, stdoutR, opts.MaxOutputBytes, cancel)
	}()
	go func() {
		defer wg.Done()
		errErr = copyLimited(&errBuf, stderrR, opts.MaxOutputBytes, cancel)
	}()

	waitErr, canceled := waitForCommand(ctx, cmd, opts.KillAfter, plainCommandSignals(cmd))
	forcedDrainClose := waitForCopies(&wg, func() {
		_ = stdoutR.Close()
		_ = stderrR.Close()
	})
	_ = stdoutR.Close()
	_ = stderrR.Close()
	waitErr = preferLimitError(waitErr, outErr, errErr)

	if outErr != nil && (!forcedDrainClose || !errors.Is(outErr, os.ErrClosed)) && waitErr == nil {
		waitErr = outErr
	}
	if errErr != nil && (!forcedDrainClose || !errors.Is(errErr, os.ErrClosed)) && waitErr == nil {
		waitErr = errErr
	}
	if canceled && parentCtx.Err() != nil && !isLimitError(waitErr) {
		waitErr = parentCtx.Err()
	}

	return outBuf.Bytes(), errBuf.Bytes(), waitErr
}

func waitForCopies(wg *sync.WaitGroup, closeReaders func()) bool {
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	timer := time.NewTimer(plainDrainTimeout)
	defer timer.Stop()
	select {
	case <-done:
		return false
	case <-timer.C:
		closeReaders()
		<-done
		return true
	}
}
