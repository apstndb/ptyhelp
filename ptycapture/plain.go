package ptycapture

import (
	"bytes"
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

	ctx, cancel := opts.context()
	defer cancel()

	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
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
		outErr = copyLimited(&outBuf, stdoutPipe, opts.MaxOutputBytes)
	}()
	go func() {
		defer wg.Done()
		errErr = copyLimited(&errBuf, stderrPipe, opts.MaxOutputBytes)
	}()

	wg.Wait()
	waitErr := cmd.Wait()
	kill()

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
