package ptycapture

import (
	"bytes"
	"context"
	"fmt"
	"io"
)

func copyLimited(dst *bytes.Buffer, src io.Reader, max int64, cancel context.CancelFunc) error {
	if max <= 0 {
		_, err := io.Copy(dst, src)
		return err
	}
	n, err := io.Copy(dst, io.LimitReader(src, max+1))
	if n > max {
		if cancel != nil {
			cancel()
		}
		return fmt.Errorf("output exceeded %d bytes", max)
	}
	return err
}
