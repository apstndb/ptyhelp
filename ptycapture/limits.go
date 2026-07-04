package ptycapture

import (
	"bytes"
	"fmt"
	"io"
)

func copyLimited(dst *bytes.Buffer, src io.Reader, max int64) error {
	if max <= 0 {
		_, err := io.Copy(dst, src)
		return err
	}
	n, err := io.Copy(dst, io.LimitReader(src, max+1))
	if n > max {
		return fmt.Errorf("output exceeded %d bytes", max)
	}
	return err
}
