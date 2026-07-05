//go:build unix

package ptycapture

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"
	"syscall"
	"time"

	"github.com/creack/pty"
	"golang.org/x/term"
)

// CapturePTY runs argv with stdin and stdout on a PTY (fixed cols×rows).
// stderr is captured with a separate pipe.
func CapturePTY(opts Options, argv []string) (stdout, stderr []byte, err error) {
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

	ctx, parentCancel := opts.context()
	defer parentCancel()
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

	kill := startKillWatcherUnix(ctx, cmd, opts.KillAfter)
	defer kill()

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

	waitErr := waitForCommand(ctx, cmd, kill)
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
	if ctx.Err() != nil && waitErr == nil {
		waitErr = ctx.Err()
	}

	return outBuf.Bytes(), errBuf.Bytes(), waitErr
}

func childStdinForPTY(tty *os.File) (*os.File, int, bool) {
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		return os.Stdin, 1, false
	}
	return tty, 0, true
}

func startKillWatcherUnix(ctx context.Context, cmd *exec.Cmd, killAfter time.Duration) func() {
	if cmd.Process == nil {
		return func() {}
	}
	pid := cmd.Process.Pid
	done := make(chan struct{})
	var once sync.Once
	stop := func() { once.Do(func() { close(done) }) }
	go func() {
		select {
		case <-ctx.Done():
			pgid, err := syscall.Getpgid(pid)
			if killAfter > 0 {
				if err == nil {
					_ = syscall.Kill(-pgid, syscall.SIGTERM)
				} else {
					_ = cmd.Process.Signal(syscall.SIGTERM)
				}
				timer := time.NewTimer(killAfter)
				select {
				case <-timer.C:
					if pgid, err := syscall.Getpgid(pid); err == nil {
						_ = syscall.Kill(-pgid, syscall.SIGKILL)
					} else {
						_ = cmd.Process.Kill()
					}
				case <-done:
					timer.Stop()
				}
			} else {
				if err == nil {
					_ = syscall.Kill(-pgid, syscall.SIGKILL)
				} else {
					_ = cmd.Process.Kill()
				}
			}
		case <-done:
		}
	}()
	return stop
}

// DrainPTYOutput reads remaining PTY output after process exit (exported for tests).
func DrainPTYOutput(r io.Reader) ([]byte, error) {
	var buf bytes.Buffer
	_, err := io.Copy(&buf, r)
	return buf.Bytes(), err
}
