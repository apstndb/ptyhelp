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
	output, err := duplicateConPTYOutput(p)
	if err != nil {
		_ = p.Close()
		return nil, nil, err
	}

	pid, handle, err := p.Spawn(argv[0], argv, nil)
	if err != nil {
		_ = output.Close()
		_ = p.Close()
		return nil, nil, err
	}
	_ = pid

	proc := windows.Handle(handle)
	defer windows.CloseHandle(proc)

	startWindowsStdin(p)

	var outBuf bytes.Buffer
	var readErr error
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		readErr = copyLimited(&outBuf, output, opts.MaxOutputBytes, cancel)
	}()

	waitErr := waitForProcess(ctx, proc, opts.KillAfter)
	closeDone := make(chan struct{})
	go func() {
		_ = p.Close()
		close(closeDone)
	}()
	wg.Wait()
	_ = output.Close()
	<-closeDone
	waitErr = preferLimitError(waitErr, readErr, nil)

	if readErr != nil && waitErr == nil {
		waitErr = readErr
	}
	if ctx.Err() != nil && waitErr == nil {
		waitErr = ctx.Err()
	}

	return outBuf.Bytes(), nil, waitErr
}

func duplicateConPTYOutput(p *conpty.ConPty) (*os.File, error) {
	// ConPty.Close closes its output handle immediately after closing the
	// pseudoconsole. Read through a duplicate so final buffered output can drain
	// to EOF even when the original handle is closed concurrently.
	process := windows.CurrentProcess()
	var output windows.Handle
	err := windows.DuplicateHandle(
		process,
		windows.Handle(p.OutPipeReadFd()),
		process,
		&output,
		0,
		false,
		windows.DUPLICATE_SAME_ACCESS,
	)
	if err != nil {
		return nil, err
	}
	return os.NewFile(uintptr(output), "conpty-output"), nil
}

func startWindowsStdin(p *conpty.ConPty) {
	go func() {
		if !term.IsTerminal(int(os.Stdin.Fd())) {
			_, _ = io.Copy(p, os.Stdin)
		}
		_ = sendConPTYEOF(p)
	}()
}

func sendConPTYEOF(p *conpty.ConPty) error {
	// Windows console input recognizes Ctrl+Z followed by Enter as EOF when
	// processed input is enabled. Closing the ConPTY input handle instead sends
	// a control-close event that terminates still-running children.
	_, err := p.Write([]byte{0x1a, '\r'})
	return err
}

func waitForProcess(ctx context.Context, h windows.Handle, killAfter time.Duration) error {
	done := make(chan error, 1)
	go func() {
		done <- waitProcess(h)
	}()

	select {
	case err := <-done:
		return err
	case <-ctx.Done():
	}

	if killAfter > 0 {
		timer := time.NewTimer(killAfter)
		select {
		case err := <-done:
			timer.Stop()
			return err
		case <-timer.C:
		}
	}

	_ = windows.TerminateProcess(h, 1)
	timer := time.NewTimer(forceKillWait)
	defer timer.Stop()
	select {
	case err := <-done:
		return err
	case <-timer.C:
		return ctx.Err()
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
