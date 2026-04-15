package main

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"strings"
)

// scannerMaxLine is the maximum bytes per line when scanning Markdown. The
// default bufio.Scanner limit is 64KiB; long single-line blobs (e.g. embedded
// JSON) would otherwise fail with bufio.ErrTooLong.
const scannerMaxLine = 16 << 20

func newLineScanner(r io.Reader) *bufio.Scanner {
	s := bufio.NewScanner(r)
	s.Buffer(make([]byte, 0, 64*1024), scannerMaxLine)
	return s
}

// patchTargetFile replaces the lines strictly between <!-- marker begin --> and <!-- marker end -->
// (exclusive of the marker lines) with a fenced ```text block containing out.
// It applies EOL normalization to the patched Markdown written back to path: eolNone matches the
// target file's perceived style (if consistent), defaulting to LF for mixed-EOL files; eolLF and
// eolCRLF normalize the entire file to LF or CRLF (not only the inserted fenced block).
func patchTargetFile(path string, out []byte, marker string, eol eolMode) error {
	if marker == "" {
		return fmt.Errorf("empty marker")
	}
	begin := fmt.Sprintf("<!-- %s begin -->", marker)
	end := fmt.Sprintf("<!-- %s end -->", marker)

	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	var hasCRLF, hasBareLF bool
	for i := 0; i < len(data); i++ {
		if data[i] == '\n' {
			if i > 0 && data[i-1] == '\r' {
				hasCRLF = true
			} else {
				hasBareLF = true
			}
		}
	}
	hasOnlyCRLF := hasCRLF && !hasBareLF

	bi, ei, err := markerLineIndices(path, data, begin, end)
	if err != nil {
		return err
	}

	// Prepare the captured output for insertion into the fenced block.
	// We normalize it to LF internally purely to simplify splitting and line
	// manipulation. Standalone \r (e.g. progress-bar overwrites) is preserved.
	// Trailing/leading whitespace and line endings from the captured output
	// are trimmed before insertion.
	//
	// Note: The final style of the inserted content will be determined by the
	// whole-file normalization at the end of this function (which respects
	// the target file's perceived style in eolNone mode to avoid mixed EOLs).
	textStr := string(bytes.TrimSpace(normalizeEOL(out, eolLF)))
	textLines := strings.Split(textStr, "\n")
	middle := append([]string{"```text"}, append(textLines, "```")...)

	var b strings.Builder
	sc := newLineScanner(bytes.NewReader(data))
	n := 0
	for sc.Scan() {
		line := sc.Text()
		switch {
		case n < bi:
			b.WriteString(line)
			b.WriteByte('\n')
		case n == bi:
			b.WriteString(line)
			b.WriteByte('\n')
			b.WriteString(strings.Join(middle, "\n"))
			b.WriteByte('\n')
		case n > bi && n < ei:
			// drop old fenced region between marker lines
		default: // n >= ei
			b.WriteString(line)
			b.WriteByte('\n')
		}
		n++
	}
	if err := sc.Err(); err != nil {
		return err
	}

	s := b.String()
	if !strings.HasSuffix(s, "\n") {
		s += "\n"
	}

	var finalOut []byte
	switch {
	case eol == eolNone && hasOnlyCRLF:
		finalOut = normalizeEOL([]byte(s), eolCRLF)
	case eol == eolNone:
		finalOut = []byte(s)
	default: // eolLF or eolCRLF
		finalOut = normalizeEOL([]byte(s), eol)
	}
	return os.WriteFile(path, finalOut, 0o644)
}

// markerLineIndices returns the line numbers (0-based, bufio.ScanLines semantics)
// of the begin and end marker lines. It avoids allocating a full CRLF→LF copy of
// the file plus a []string of every line; only one line is held at a time.
func markerLineIndices(path string, data []byte, begin, end string) (bi, ei int, err error) {
	bi, ei = -1, -1
	sc := newLineScanner(bytes.NewReader(data))
	n := 0
	for sc.Scan() {
		line := sc.Text()
		if strings.Contains(line, begin) {
			bi = n
			n++
			continue
		}
		if bi >= 0 && strings.Contains(line, end) {
			ei = n
			break
		}
		n++
	}
	if scanErr := sc.Err(); scanErr != nil {
		return -1, -1, scanErr
	}
	if bi < 0 || ei < 0 || ei <= bi {
		return -1, -1, fmt.Errorf("%s: %q … %q block not found or invalid order", path, begin, end)
	}
	return bi, ei, nil
}
