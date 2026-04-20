//go:build unix && !linux && !darwin

package main

import "os"

func disableTTYEcho(*os.File) error {
	return nil
}
