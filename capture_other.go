//go:build !unix

package main

import (
	"fmt"
	"io"

	gpty "github.com/aymanbagabas/go-pty"
)

// capturePTY runs argv in a single PTY (combined stream). On non-Unix platforms
// we do not split stdout/stderr; the captured bytes are returned as stdout only so
// callers can keep a single code path for writing to file or patching.
func capturePTY(cols, rows uint, argv []string) (stdout, stderr []byte, err error) {
	if len(argv) == 0 {
		return nil, nil, fmt.Errorf("empty command")
	}
	if cols > 0xffff || rows > 0xffff {
		return nil, nil, fmt.Errorf("cols/rows out of range")
	}

	p, err := gpty.New()
	if err != nil {
		return nil, nil, err
	}
	defer p.Close()

	if err := p.Resize(int(cols), int(rows)); err != nil {
		return nil, nil, err
	}

	cmd := p.Command(argv[0], argv[1:]...)

	if err := cmd.Start(); err != nil {
		return nil, nil, err
	}

	out, readErr := io.ReadAll(p)
	waitErr := cmd.Wait()
	if readErr != nil && waitErr == nil {
		waitErr = readErr
	}
	return out, nil, waitErr
}
