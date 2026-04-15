//go:build unix

package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"
	"syscall"

	"github.com/creack/pty"
)

// capturePTY runs argv with stdin and stdout on a PTY (fixed cols×rows) so tools that read
// terminal size from fd 1 wrap predictably. stderr is captured with a separate pipe.
func capturePTY(cols, rows uint, argv []string) (stdout, stderr []byte, err error) {
	if len(argv) == 0 {
		return nil, nil, fmt.Errorf("empty command")
	}
	if cols > 0xffff || rows > 0xffff {
		return nil, nil, fmt.Errorf("cols/rows out of range")
	}

	master, tty, err := pty.Open()
	if err != nil {
		return nil, nil, err
	}
	defer func() { _ = master.Close() }()
	defer func() { _ = tty.Close() }()

	ws := &pty.Winsize{Rows: uint16(rows), Cols: uint16(cols)}
	if err := pty.Setsize(tty, ws); err != nil {
		return nil, nil, err
	}

	stderrR, stderrW, err := os.Pipe()
	if err != nil {
		return nil, nil, err
	}

	cmd := exec.Command(argv[0], argv[1:]...)
	cmd.Stdin = tty
	cmd.Stdout = tty
	cmd.Stderr = stderrW
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setsid:  true,
		Setctty: true,
		// Setctty expects the descriptor number in the child process, not the
		// parent's tty FD value. stdin is attached to tty as child fd 0.
		Ctty: 0,
	}

	if err := cmd.Start(); err != nil {
		_ = stderrR.Close()
		_ = stderrW.Close()
		return nil, nil, err
	}
	_ = tty.Close()
	_ = stderrW.Close()

	var wg sync.WaitGroup
	var outBuf, errBuf bytes.Buffer
	var outErr, errErr error
	wg.Add(2)
	go func() {
		defer wg.Done()
		_, outErr = io.Copy(&outBuf, master)
	}()
	go func() {
		defer wg.Done()
		_, errErr = io.Copy(&errBuf, stderrR)
	}()

	waitErr := cmd.Wait()
	wg.Wait()
	_ = master.Close()
	_ = stderrR.Close()

	if outErr != nil && !errors.Is(outErr, syscall.EIO) && !errors.Is(outErr, os.ErrClosed) {
		if waitErr == nil {
			waitErr = outErr
		}
	}
	if errErr != nil && !errors.Is(errErr, os.ErrClosed) && waitErr == nil {
		waitErr = errErr
	}

	return outBuf.Bytes(), errBuf.Bytes(), waitErr
}
