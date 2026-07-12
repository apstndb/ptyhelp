package ptycapture

import (
	"fmt"
	"time"
)

// StderrMode controls how stderr is exposed to callers.
type StderrMode int

const (
	// StderrSeparate keeps stdout and stderr in separate slices.
	StderrSeparate StderrMode = iota
	// StderrMerge appends stderr bytes after stdout without preserving temporal ordering.
	StderrMerge
	// StderrDiscard drops captured stderr.
	StderrDiscard
)

// ParseStderrMode parses -stderr flag values.
func ParseStderrMode(s string) (StderrMode, error) {
	switch s {
	case "", "separate":
		return StderrSeparate, nil
	case "merge":
		return StderrMerge, nil
	case "discard":
		return StderrDiscard, nil
	default:
		return 0, fmt.Errorf("invalid stderr mode %q (valid: separate, merge, discard)", s)
	}
}

// ApplyStderrMode merges or discards stderr per mode. Merge appends the complete
// stderr stream after stdout; it does not preserve the streams' temporal ordering.
func ApplyStderrMode(stdout, stderr []byte, mode StderrMode) ([]byte, []byte) {
	switch mode {
	case StderrMerge:
		if len(stderr) == 0 {
			return stdout, nil
		}
		return append(stdout, stderr...), nil
	case StderrDiscard:
		return stdout, nil
	default:
		return stdout, stderr
	}
}

// Options configures subprocess capture.
type Options struct {
	Cols uint
	Rows uint
	// KillAfter allows graceful termination before forcefully stopping the process.
	KillAfter time.Duration
	// MaxOutputBytes limits each captured output stream when greater than zero.
	MaxOutputBytes int64
}
