//go:build !unix && !windows

package ptycapture

import (
	"context"
	"fmt"
)

// CapturePTY is not supported on this platform.
func CapturePTY(_ context.Context, _ Options, _ []string) (stdout, stderr []byte, err error) {
	return nil, nil, fmt.Errorf("PTY capture is not supported on this platform")
}
