package main

import (
	"bytes"
	"fmt"
)

// eolMode represents a line-ending normalization mode.
type eolMode int

const (
	eolNone eolMode = iota
	eolLF
	eolCRLF
)

// parseEOLMode parses a string into an eolMode.
// Valid values are "none", "lf", and "crlf" (case-sensitive).
func parseEOLMode(s string) (eolMode, error) {
	switch s {
	case "none":
		return eolNone, nil
	case "lf":
		return eolLF, nil
	case "crlf":
		return eolCRLF, nil
	default:
		return eolNone, fmt.Errorf("invalid -normalize-eol value %q (valid: none, lf, crlf)", s)
	}
}

// normalizeEOL applies the given line-ending normalization to data.
// eolNone returns data unchanged.
func normalizeEOL(data []byte, mode eolMode) []byte {
	switch mode {
	case eolLF:
		return bytes.ReplaceAll(data, []byte("\r\n"), []byte("\n"))
	case eolCRLF:
		// First normalize to LF so we don't double up existing CRLF sequences,
		// then convert all LF to CRLF.
		tmp := bytes.ReplaceAll(data, []byte("\r\n"), []byte("\n"))
		return bytes.ReplaceAll(tmp, []byte("\n"), []byte("\r\n"))
	default:
		return data
	}
}
