//go:build windows

package ptycapture

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"sync"
	"time"

	"github.com/charmbracelet/x/conpty"
	"golang.org/x/sys/windows"
	"golang.org/x/term"
)

// CapturePTY runs argv in a Windows ConPTY (combined stdout/stderr stream).
func CapturePTY(opts Options, argv []string) (stdout, stderr []byte, err error) {
	if len(argv) == 0 {
		return nil, nil, fmt.Errorf("empty command")
	}
	if opts.Cols == 0 || opts.Rows == 0 {
		return nil, nil, fmt.Errorf("cols and rows must be >= 1")
	}
	if opts.Cols > 0xffff || opts.Rows > 0xffff {
		return nil, nil, fmt.Errorf("cols/rows out of range")
	}
	cols, rows := int(opts.Cols), int(opts.Rows)

	ctx, parentCancel := opts.context()
	defer parentCancel()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	p, err := conpty.New(cols, rows, 0)
	if err != nil {
		return nil, nil, err
	}
	defer p.Close()

	pid, handle, err := p.Spawn(argv[0], argv, nil)
	if err != nil {
		return nil, nil, err
	}
	_ = pid

	proc := windows.Handle(handle)
	defer windows.CloseHandle(proc)

	startWindowsStdin(p)

	stopKill := startKillWatcherWindows(ctx, proc)

	var outBuf bytes.Buffer
	var readErr error
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		readErr = copyLimited(&outBuf, p, opts.MaxOutputBytes, cancel)
	}()

	waitErr := waitForProcess(ctx, proc, stopKill)
	wg.Wait()
	waitErr = preferLimitError(waitErr, readErr, nil)

	if readErr != nil && waitErr == nil {
		waitErr = readErr
	}
	if ctx.Err() != nil && waitErr == nil {
		waitErr = ctx.Err()
	}

	return outBuf.Bytes(), nil, waitErr
}

func startWindowsStdin(p *conpty.ConPty) {
	go func() {
		if term.IsTerminal(int(os.Stdin.Fd())) {
			_ = closeConPTYStdin(p)
			return
		}
		_, _ = io.Copy(p, os.Stdin)
		_ = closeConPTYStdin(p)
	}()
}

func closeConPTYStdin(p *conpty.ConPty) error {
	return windows.CloseHandle(windows.Handle(p.InPipeWriteFd()))
}

func startKillWatcherWindows(ctx context.Context, proc windows.Handle) func() {
	done := make(chan struct{})
	var once sync.Once
	stop := func() { once.Do(func() { close(done) }) }
	go func() {
		select {
		case <-ctx.Done():
			_ = windows.TerminateProcess(proc, 1)
		case <-done:
		}
	}()
	return stop
}

func waitForProcess(ctx context.Context, h windows.Handle, stopKill func()) error {
	done := make(chan error, 1)
	go func() {
		done <- waitProcess(h)
	}()

	select {
	case err := <-done:
		stopKill()
		return err
	case <-ctx.Done():
		_ = windows.TerminateProcess(h, 1)
		select {
		case err := <-done:
			stopKill()
			return err
		case <-time.After(5 * time.Second):
			stopKill()
			return ctx.Err()
		}
	}
}

func waitProcess(h windows.Handle) error {
	s, err := windows.WaitForSingleObject(h, windows.INFINITE)
	if err != nil {
		return err
	}
	if s != windows.WAIT_OBJECT_0 {
		return fmt.Errorf("wait for process: unexpected status %d", s)
	}
	var code uint32
	if err := windows.GetExitCodeProcess(h, &code); err != nil {
		return err
	}
	if code != 0 {
		return &exitStatusError{code: int(code)}
	}
	return nil
}
