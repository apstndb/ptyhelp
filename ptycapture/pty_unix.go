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

	ctx, cancel := opts.context()
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

	stderrR, stderrW, err := os.Pipe()
	if err != nil {
		return nil, nil, err
	}

	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	cmd.Stdin = tty
	cmd.Stdout = tty
	cmd.Stderr = stderrW
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setsid:  true,
		Setctty: true,
		Ctty:    0,
	}

	if err := cmd.Start(); err != nil {
		_ = stderrR.Close()
		_ = stderrW.Close()
		return nil, nil, err
	}
	_ = tty.Close()
	_ = stderrW.Close()

	kill := startKillWatcherUnix(ctx, cmd, opts.KillAfter)

	var wg sync.WaitGroup
	var outBuf, errBuf bytes.Buffer
	var outErr, errErr error
	wg.Add(2)
	go func() {
		defer wg.Done()
		outErr = copyLimited(&outBuf, master, opts.MaxOutputBytes)
	}()
	go func() {
		defer wg.Done()
		errErr = copyLimited(&errBuf, stderrR, opts.MaxOutputBytes)
	}()

	waitErr := cmd.Wait()
	wg.Wait()
	kill()
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
	if ctx.Err() != nil && waitErr == nil {
		waitErr = ctx.Err()
	}

	return outBuf.Bytes(), errBuf.Bytes(), waitErr
}

func startKillWatcherUnix(ctx context.Context, cmd *exec.Cmd, killAfter time.Duration) func() {
	if cmd.Process == nil {
		return func() {}
	}
	pid := cmd.Process.Pid
	done := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			pgid, err := syscall.Getpgid(pid)
			if err == nil {
				_ = syscall.Kill(-pgid, syscall.SIGTERM)
			} else {
				_ = cmd.Process.Kill()
			}
			if killAfter > 0 {
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
			}
		case <-done:
		}
	}()
	return func() { close(done) }
}

// DrainPTYOutput reads remaining PTY output after process exit (exported for tests).
func DrainPTYOutput(r io.Reader) ([]byte, error) {
	var buf bytes.Buffer
	_, err := io.Copy(&buf, r)
	return buf.Bytes(), err
}
