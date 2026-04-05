package main

import (
	"fmt"
	"io"
	"os"

	gpty "github.com/aymanbagabas/go-pty"
)

// captureCommand runs argv in a PTY with the given dimensions and returns combined output.
// argv[0] is the executable name; argv[1:] are arguments. The subprocess uses the caller's
// current working directory (cmd.Dir is left unset).
func captureCommand(cols, rows uint, argv []string) ([]byte, error) {
	if len(argv) == 0 {
		return nil, fmt.Errorf("empty command")
	}
	if cols > 0xffff || rows > 0xffff {
		return nil, fmt.Errorf("cols/rows out of range")
	}

	p, err := gpty.New()
	if err != nil {
		return nil, err
	}
	defer p.Close()

	if err := p.Resize(int(cols), int(rows)); err != nil {
		return nil, err
	}

	cmd := p.Command(argv[0], argv[1:]...)
	cmd.Env = os.Environ()

	if err := cmd.Start(); err != nil {
		return nil, err
	}

	out, err := io.ReadAll(p)
	waitErr := cmd.Wait()
	if err != nil {
		return nil, err
	}
	_ = waitErr
	return out, nil
}
