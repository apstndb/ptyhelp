//go:build !unix && !windows

package ptycapture

import "fmt"

// CapturePTY is not supported on this platform.
func CapturePTY(opts Options, argv []string) (stdout, stderr []byte, err error) {
	return nil, nil, fmt.Errorf("PTY capture is not supported on this platform")
}
