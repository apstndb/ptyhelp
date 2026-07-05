package ptycapture

import (
	"context"
	"fmt"
	"time"
)

// StderrMode controls how stderr is exposed to callers.
type StderrMode int

const (
	// StderrSeparate keeps stdout and stderr in separate slices.
	StderrSeparate StderrMode = iota
	// StderrMerge appends stderr bytes onto stdout.
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

// ApplyStderrMode merges or discards stderr per mode.
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
	Cols           uint
	Rows           uint
	Ctx            context.Context
	Timeout        time.Duration
	KillAfter      time.Duration
	MaxOutputBytes int64
}

func (o Options) context() (context.Context, context.CancelFunc) {
	if o.Ctx != nil {
		if o.Timeout > 0 {
			return context.WithTimeout(o.Ctx, o.Timeout)
		}
		return o.Ctx, func() {}
	}
	if o.Timeout > 0 {
		return context.WithTimeout(context.Background(), o.Timeout)
	}
	return context.Background(), func() {}
}
