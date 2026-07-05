package mdpatch

import (
	"bytes"
	"fmt"
)

// EOLMode represents a line-ending normalization mode.
type EOLMode int

const (
	// EOLNone leaves line endings unchanged except where patch logic applies its own rules.
	EOLNone EOLMode = iota
	// EOLLF normalizes all line endings to LF.
	EOLLF
	// EOLCRLF normalizes all line endings to CRLF.
	EOLCRLF
)

// ParseEOLMode parses a string into an EOLMode.
// Valid values are "none", "lf", and "crlf" (case-sensitive).
func ParseEOLMode(s string) (EOLMode, error) {
	switch s {
	case "none":
		return EOLNone, nil
	case "lf":
		return EOLLF, nil
	case "crlf":
		return EOLCRLF, nil
	default:
		return EOLNone, fmt.Errorf("invalid EOL mode %q (valid: none, lf, crlf)", s)
	}
}

// NormalizeEOL applies the given line-ending normalization to data.
// EOLNone returns data unchanged.
func NormalizeEOL(data []byte, mode EOLMode) []byte {
	switch mode {
	case EOLLF:
		return bytes.ReplaceAll(data, []byte("\r\n"), []byte("\n"))
	case EOLCRLF:
		tmp := bytes.ReplaceAll(data, []byte("\r\n"), []byte("\n"))
		return bytes.ReplaceAll(tmp, []byte("\n"), []byte("\r\n"))
	default:
		return data
	}
}
