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
	termiosCopy := *termios
	termiosCopy.Lflag &^= unix.ECHO
	return unix.IoctlSetTermios(int(f.Fd()), unix.TCSETS, &termiosCopy)
}

func ptyEOFByte(f *os.File) (byte, error) {
	termios, err := unix.IoctlGetTermios(int(f.Fd()), unix.TCGETS)
	if err != nil {
		return 0, err
	}
	return termios.Cc[unix.VEOF], nil
}
