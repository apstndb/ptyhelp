package ptycapture

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"sync"
)

// CapturePlain runs argv with ordinary pipe I/O (no pseudo-terminal).
func CapturePlain(opts Options, argv []string) (stdout, stderr []byte, err error) {
	if len(argv) == 0 {
		return nil, nil, fmt.Errorf("empty command")
	}

	ctx, parentCancel := opts.context()
	defer parentCancel()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	cmd.Cancel = func() error { return nil }
	cmd.Stdin = os.Stdin

	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return nil, nil, err
	}
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return nil, nil, err
	}

	if err := cmd.Start(); err != nil {
		return nil, nil, err
	}

	kill := startKillWatcher(ctx, cmd, opts.KillAfter)

	var wg sync.WaitGroup
	var outBuf, errBuf bytes.Buffer
	var outErr, errErr error
	wg.Add(2)
	go func() {
		defer wg.Done()
		outErr = copyLimited(&outBuf, stdoutPipe, opts.MaxOutputBytes, cancel)
	}()
	go func() {
		defer wg.Done()
		errErr = copyLimited(&errBuf, stderrPipe, opts.MaxOutputBytes, cancel)
	}()

	waitErr := waitForCommand(ctx, cmd, kill)
	wg.Wait()
	waitErr = preferLimitError(waitErr, outErr, errErr)

	if outErr != nil && waitErr == nil {
		waitErr = outErr
	}
	if errErr != nil && waitErr == nil {
		waitErr = errErr
	}
	if ctx.Err() != nil && waitErr == nil {
		waitErr = ctx.Err()
	}

	return outBuf.Bytes(), errBuf.Bytes(), waitErr
}
