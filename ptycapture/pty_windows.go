//go:build windows

package ptycapture

import (
	"bytes"
	"fmt"
	"time"

	"github.com/charmbracelet/x/conpty"
	"golang.org/x/sys/windows"
)

// CapturePTY runs argv in a Windows ConPTY (combined stdout/stderr stream).
func CapturePTY(opts Options, argv []string) (stdout, stderr []byte, err error) {
	if len(argv) == 0 {
		return nil, nil, fmt.Errorf("empty command")
	}
	cols, rows := int(opts.Cols), int(opts.Rows)
	if cols == 0 || rows == 0 {
		return nil, nil, fmt.Errorf("cols and rows must be >= 1")
	}
	if cols > 0xffff || rows > 0xffff {
		return nil, nil, fmt.Errorf("cols/rows out of range")
	}

	ctx, cancel := opts.context()
	defer cancel()

	p, err := conpty.New(cols, rows, 0)
	if err != nil {
		return nil, nil, err
	}
	defer p.Close()

	pid, handle, err := p.Spawn(argv[0], argv[1:], nil)
	if err != nil {
		return nil, nil, err
	}
	_ = pid

	proc := windows.Handle(handle)
	defer windows.CloseHandle(proc)

	killDone := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			_ = windows.TerminateProcess(proc, 1)
			if opts.KillAfter > 0 {
				timer := time.NewTimer(opts.KillAfter)
				select {
				case <-timer.C:
					_ = windows.TerminateProcess(proc, 1)
				case <-killDone:
					timer.Stop()
				}
			}
		case <-killDone:
		}
	}()

	var outBuf bytes.Buffer
	readErr := copyLimited(&outBuf, p, opts.MaxOutputBytes)

	waitErr := waitProcess(proc)
	close(killDone)

	if readErr != nil && waitErr == nil {
		waitErr = readErr
	}
	if ctx.Err() != nil && waitErr == nil {
		waitErr = ctx.Err()
	}

	return outBuf.Bytes(), nil, waitErr
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
