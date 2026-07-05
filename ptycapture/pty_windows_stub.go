//go:build windows

package ptycapture

import "fmt"

// CapturePTY is implemented in PR #20 (ConPTY). This stub keeps PR #18 buildable on Windows CI.
func CapturePTY(opts Options, argv []string) (stdout, stderr []byte, err error) {
	return nil, nil, fmt.Errorf("PTY capture is not supported on this platform")
}
