//go:build linux

package main

import (
	"os"

	"golang.org/x/sys/unix"
)

func disableTTYEcho(f *os.File) error {
	termios, err := unix.IoctlGetTermios(int(f.Fd()), unix.TCGETS)
	if err != nil {
		return err
	}
	copy := *termios
	copy.Lflag &^= unix.ECHO
	return unix.IoctlSetTermios(int(f.Fd()), unix.TCSETS, &copy)
}
