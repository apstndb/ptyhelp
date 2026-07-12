//go:build unix

package ptycapture

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"sync"
	"syscall"

	"github.com/creack/pty"
	"golang.org/x/term"
)

// CapturePTY runs argv with stdin and stdout on a PTY (fixed cols×rows).
// stderr is captured with a separate pipe. A nil context is treated as
// context.Background.
func CapturePTY(ctx context.Context, opts Options, argv []string) (stdout, stderr []byte, err error) {
	if len(argv) == 0 {
		return nil, nil, fmt.Errorf("empty command")
	}
	cols, rows := opts.Cols, opts.Rows
	if cols == 0 || rows == 0 {
		return nil, nil, fmt.Errorf("cols and rows must be >= 1")
	}
	if cols > 0xffff || rows > 0xffff {
		return nil, nil, fmt.Errorf("cols/rows out of range")
	}

	if ctx == nil {
		ctx = context.Background()
	}
	parentCtx := ctx
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

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

	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	cmd.Cancel = func() error { return nil }
	cmd.Stdin = stdin
	cmd.Stdout = tty
	cmd.Stderr = stderrW
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setsid:  true,
		Setctty: true,
		Ctty:    ctty,
	}

	if err := cmd.Start(); err != nil {
		_ = stderrR.Close()
		_ = stderrW.Close()
		return nil, nil, err
	}

	var wg sync.WaitGroup
	if sendEOF {
		eofByte := byte(4)
		if configuredEOF, eofErr := ptyEOFByte(tty); eofErr == nil {
			eofByte = configuredEOF
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = master.Write([]byte{eofByte})
		}()
	}
	_ = tty.Close()
	_ = stderrW.Close()

	var outBuf, errBuf bytes.Buffer
	var outErr, errErr error
	wg.Add(2)
	go func() {
		defer wg.Done()
		outErr = copyLimited(&outBuf, master, opts.MaxOutputBytes, cancel)
	}()
	go func() {
		defer wg.Done()
		errErr = copyLimited(&errBuf, stderrR, opts.MaxOutputBytes, cancel)
	}()

	waitErr, canceled := waitForCommand(ctx, cmd, opts.KillAfter, unixCommandSignals(cmd))
	_ = master.Close()
	_ = stderrR.Close()
	wg.Wait()
	waitErr = preferLimitError(waitErr, outErr, errErr)

	if outErr != nil && !errors.Is(outErr, syscall.EIO) && !errors.Is(outErr, os.ErrClosed) {
		if waitErr == nil {
			waitErr = outErr
		}
	}
	if errErr != nil && !errors.Is(errErr, os.ErrClosed) && waitErr == nil {
		waitErr = errErr
	}
	if canceled && parentCtx.Err() != nil && !isLimitError(waitErr) {
		waitErr = parentCtx.Err()
	}

	return outBuf.Bytes(), errBuf.Bytes(), waitErr
}

func childStdinForPTY(tty *os.File) (*os.File, int, bool) {
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		return os.Stdin, 1, false
	}
	return tty, 0, true
}
