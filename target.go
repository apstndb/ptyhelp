package main

import (
	"bytes"
	"fmt"
	"os"
	"strings"
)

// patchTargetFile replaces the lines strictly between <!-- marker begin --> and <!-- marker end -->
// (exclusive of the marker lines) with a fenced ```text block containing out.
func patchTargetFile(path string, out []byte, marker string) error {
	if marker == "" {
		return fmt.Errorf("empty marker")
	}
	begin := fmt.Sprintf("<!-- %s begin -->", marker)
	end := fmt.Sprintf("<!-- %s end -->", marker)

	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	// Normalize CRLF for parsing; write back with LF only.
	raw := string(bytes.ReplaceAll(data, []byte("\r\n"), []byte("\n")))
	lines := strings.Split(raw, "\n")

	var bi, ei = -1, -1
	for i, line := range lines {
		if strings.Contains(line, begin) {
			bi = i
			continue
		}
		if bi >= 0 && strings.Contains(line, end) {
			ei = i
			break
		}
	}
	if bi < 0 || ei < 0 || ei <= bi {
		return fmt.Errorf("%s: %q … %q block not found or invalid order", path, begin, end)
	}

	text := strings.TrimRight(string(bytes.TrimSpace(out)), "\n")
	textLines := strings.Split(text, "\n")
	middle := append([]string{"```text"}, append(textLines, "```")...)

	outLines := append(append(append([]string{}, lines[:bi+1]...), middle...), lines[ei:]...)
	s := strings.Join(outLines, "\n")
	if !strings.HasSuffix(s, "\n") {
		s += "\n"
	}
	return os.WriteFile(path, []byte(s), 0o644)
}
