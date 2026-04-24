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
	"golang.org/x/term"
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
	if err := disableTTYEcho(tty); err != nil {
		return nil, nil, err
	}

	stderrR, stderrW, err := os.Pipe()
	if err != nil {
		return nil, nil, err
	}
	stdin, ctty, sendEOF := childStdinForPTY(tty)

	cmd := exec.Command(argv[0], argv[1:]...)
	cmd.Stdin = stdin
	cmd.Stdout = tty
	cmd.Stderr = stderrW
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setsid:  true,
		Setctty: true,
		// Setctty expects the descriptor number in the child process, not the
		// parent's tty FD value. childStdinForPTY selects which child fd refers
		// to the slave PTY in this process layout.
		Ctty: ctty,
	}

	if err := cmd.Start(); err != nil {
		_ = stderrR.Close()
		_ = stderrW.Close()
		return nil, nil, err
	}

	var wg sync.WaitGroup
	var outBuf, errBuf bytes.Buffer
	var outErr, errErr error

	if sendEOF {
		eofByte := byte(4)
		if configuredEOF, eofErr := ptyEOFByte(tty); eofErr == nil {
			eofByte = configuredEOF
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			// In PTY mode we buffer output rather than acting as an interactive
			// terminal. When stdin is not redirected, inject the PTY's configured
			// EOF character so stdin-reading commands do not hang forever.
			_, _ = master.Write([]byte{eofByte})
		}()
	}
	_ = tty.Close()
	_ = stderrW.Close()

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
	_ = master.Close()
	_ = stderrR.Close()
	wg.Wait()

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

// childStdinForPTY selects the child's stdin source and controlling-terminal fd
// based on whether the parent stdin is an actual terminal.
func childStdinForPTY(tty *os.File) (*os.File, int, bool) {
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		// Preserve redirected stdin bytes exactly; keep the PTY attached via stdout.
		return os.Stdin, 1, false
	}
	return tty, 0, true
}
