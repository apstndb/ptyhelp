package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"
)

// capturePlain runs argv with ordinary pipe I/O (no pseudo-terminal). The environment
// is inherited from the parent; set COLUMNS/LINES yourself (shell, env) if a child needs them.
func capturePlain(argv []string) (stdout, stderr []byte, err error) {
	if len(argv) == 0 {
		return nil, nil, fmt.Errorf("empty command")
	}

	cmd := exec.Command(argv[0], argv[1:]...)
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

	var wg sync.WaitGroup
	var outBuf, errBuf bytes.Buffer
	var outErr, errErr error
	wg.Add(2)
	go func() {
		defer wg.Done()
		_, outErr = io.Copy(&outBuf, stdoutPipe)
	}()
	go func() {
		defer wg.Done()
		_, errErr = io.Copy(&errBuf, stderrPipe)
	}()

	// Complete all reads before Wait (StdoutPipe/StderrPipe requirement).
	wg.Wait()
	waitErr := cmd.Wait()

	if outErr != nil && waitErr == nil {
		waitErr = outErr
	}
	if errErr != nil && waitErr == nil {
		waitErr = errErr
	}

	return outBuf.Bytes(), errBuf.Bytes(), waitErr
}
