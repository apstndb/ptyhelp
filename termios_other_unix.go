//go:build unix && !linux && !darwin

package main

import "os"

func disableTTYEcho(*os.File) error {
	return nil
}

func ptyEOFByte(*os.File) (byte, error) {
	return 4, nil
}
