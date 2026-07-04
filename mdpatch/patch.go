package mdpatch

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"unicode"
)

// scannerMaxLine is the maximum bytes per line when scanning Markdown.
const scannerMaxLine = 16 << 20

func newLineScanner(r io.Reader) *bufio.Scanner {
	s := bufio.NewScanner(r)
	s.Buffer(make([]byte, 0, 64*1024), scannerMaxLine)
	return s
}

// PatchOptions configures marker-block replacement.
type PatchOptions struct {
	EOL   EOLMode
	Fence string // "text", "none", or a fenced-code language tag (e.g. "go")
}

// ParseFence parses -fence values: text, none, or a language tag.
func ParseFence(s string) (string, error) {
	if s == "" {
		return "text", nil
	}
	if s == "text" || s == "none" {
		return s, nil
	}
	for _, r := range s {
		if r <= unicode.MaxASCII && (unicode.IsLetter(r) || unicode.IsDigit(r) || r == '-' || r == '_') {
			continue
		}
		return "", fmt.Errorf("invalid -fence value %q (valid: text, none, or a language tag)", s)
	}
	return s, nil
}

// ValidateMarker checks that marker is safe to embed in HTML comment markers.
func ValidateMarker(marker string) error {
	if marker == "" {
		return fmt.Errorf("empty marker")
	}
	for _, r := range marker {
		if r <= unicode.MaxASCII && (unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' || r == '.' || r == ':' || r == '-') {
			continue
		}
		return fmt.Errorf("invalid marker %q (use letters, digits, _, ., :, -)", marker)
	}
	return nil
}

// BuildPatchedContent returns the file bytes after replacing the marker block
// without writing to disk.
func BuildPatchedContent(path string, data []byte, marker string, opts PatchOptions) ([]byte, error) {
	if err := ValidateMarker(marker); err != nil {
		return nil, err
	}
	begin := fmt.Sprintf("<!-- %s begin -->", marker)
	end := fmt.Sprintf("<!-- %s end -->", marker)

	orig, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	hasCRLF, hasBareLF := detectEOLStyle(orig)

	bi, ei, err := markerLineIndices(path, orig, begin, end)
	if err != nil {
		return nil, err
	}

	normalized := NormalizeEOL(data, EOLLF)
	textStr := string(bytes.TrimRight(normalized, "\n"))
	middle := buildMiddleLines(textStr, opts.Fence)

	var b strings.Builder
	sc := newLineScanner(bytes.NewReader(orig))
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
			// drop old region between marker lines
		default:
			b.WriteString(line)
			b.WriteByte('\n')
		}
		n++
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}

	s := b.String()
	if !strings.HasSuffix(s, "\n") {
		s += "\n"
	}

	switch {
	case opts.EOL == EOLNone && hasCRLF && !hasBareLF:
		return NormalizeEOL([]byte(s), EOLCRLF), nil
	case opts.EOL == EOLNone:
		return []byte(s), nil
	default:
		return NormalizeEOL([]byte(s), opts.EOL), nil
	}
}

// PatchMarkdownFile replaces the lines strictly between <!-- marker begin -->
// and <!-- marker end --> with fenced or raw content per opts.Fence.
func PatchMarkdownFile(path string, out []byte, marker string, opts PatchOptions) error {
	content, err := BuildPatchedContent(path, out, marker, opts)
	if err != nil {
		return err
	}
	return atomicWriteFile(path, content)
}

func buildMiddleLines(textStr, fence string) []string {
	textLines := strings.Split(textStr, "\n")
	if fence == "none" {
		return textLines
	}
	lang := fence
	if lang == "" || lang == "text" {
		lang = "text"
	}
	f := fenceForContent(textStr)
	openFence := f + lang
	closeFence := f
	return append([]string{openFence}, append(textLines, closeFence)...)
}

func detectEOLStyle(data []byte) (hasCRLF, hasBareLF bool) {
	for i := 0; i < len(data); i++ {
		if data[i] == '\n' {
			if i > 0 && data[i-1] == '\r' {
				hasCRLF = true
			} else {
				hasBareLF = true
			}
		}
	}
	return hasCRLF, hasBareLF
}

func longestBacktickRun(s string) int {
	max, cur := 0, 0
	for _, r := range s {
		if r == '`' {
			cur++
			if cur > max {
				max = cur
			}
		} else {
			cur = 0
		}
	}
	return max
}

func fenceForContent(s string) string {
	n := longestBacktickRun(s) + 1
	if n < 3 {
		n = 3
	}
	return strings.Repeat("`", n)
}

func markerLineIndices(path string, data []byte, begin, end string) (bi, ei int, err error) {
	bi, ei = -1, -1
	sc := newLineScanner(bytes.NewReader(data))
	n := 0
	for sc.Scan() {
		line := sc.Text()
		trimmed := strings.TrimSpace(line)
		switch trimmed {
		case begin:
			if bi >= 0 {
				if ei >= 0 {
					return -1, -1, fmt.Errorf("%s: duplicate marker block %q", path, begin)
				}
				return -1, -1, fmt.Errorf("%s: duplicate begin marker %q", path, begin)
			}
			bi = n
		case end:
			if bi < 0 {
				return -1, -1, fmt.Errorf("%s: %q … %q block not found or invalid order", path, begin, end)
			}
			if ei >= 0 {
				return -1, -1, fmt.Errorf("%s: duplicate end marker %q", path, end)
			}
			ei = n
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

func atomicWriteFile(path string, data []byte) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, filepath.Base(path)+".tmp-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	removeTmp := true
	defer func() {
		if removeTmp {
			_ = os.Remove(tmpName)
		}
	}()

	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}

	mode := os.FileMode(0o644)
	if info, statErr := os.Stat(path); statErr == nil {
		mode = info.Mode().Perm()
	}
	if err := os.Chmod(tmpName, mode); err != nil {
		return err
	}
	if err := os.Rename(tmpName, path); err != nil {
		return err
	}
	removeTmp = false
	return nil
}
