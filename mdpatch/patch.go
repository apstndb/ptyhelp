package mdpatch

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"unicode"
)

// scannerMaxLine is the maximum bytes per line when scanning Markdown.
const scannerMaxLine = 16 << 20

var (
	// ErrMarkerNotFound indicates that a complete marker block was not found.
	ErrMarkerNotFound = errors.New("marker block not found")
	// ErrDuplicateMarker indicates that a marker line occurs more than once.
	ErrDuplicateMarker = errors.New("duplicate marker")
	// ErrInvalidMarker indicates that a marker name cannot be represented safely.
	ErrInvalidMarker = errors.New("invalid marker")
	// ErrInvalidFence indicates that a fence language tag is invalid.
	ErrInvalidFence = errors.New("invalid fence")
)

func newLineScanner(r io.Reader) *bufio.Scanner {
	s := bufio.NewScanner(r)
	s.Buffer(make([]byte, 0, 64*1024), scannerMaxLine)
	return s
}

// PatchOptions configures marker-block replacement.
type PatchOptions struct {
	EOL   EOLMode
	Fence Fence
}

// Fence controls fenced-code wrapping. Its zero value is FenceText.
type Fence string

const (
	// FenceText emits a fenced code block tagged as text.
	FenceText Fence = "text"
	// FenceNone emits replacement content without a fenced code block.
	FenceNone Fence = "none"
)

// ParseFence parses -fence values: text, none, or a language tag.
func ParseFence(s string) (Fence, error) {
	if s == "" {
		return FenceText, nil
	}
	if s == "text" || s == "none" {
		return Fence(s), nil
	}
	for _, r := range s {
		if r <= unicode.MaxASCII && (unicode.IsLetter(r) || unicode.IsDigit(r) || strings.ContainsRune("-_+#.", r)) {
			continue
		}
		return "", fmt.Errorf("%w value %q (valid: text, none, or a language tag)", ErrInvalidFence, s)
	}
	return Fence(s), nil
}

// ValidateMarker checks that marker is safe to embed in HTML comment markers.
func ValidateMarker(marker string) error {
	if marker == "" {
		return fmt.Errorf("%w: empty name", ErrInvalidMarker)
	}
	if strings.Contains(marker, "--") {
		return fmt.Errorf("%w %q (must not contain --)", ErrInvalidMarker, marker)
	}
	for _, r := range marker {
		if r <= unicode.MaxASCII && (unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' || r == '.' || r == ':' || r == '-') {
			continue
		}
		return fmt.Errorf("%w %q (use letters, digits, _, ., :, -)", ErrInvalidMarker, marker)
	}
	return nil
}

// PatchBytes returns original after replacing the marker block with replacement,
// without performing file I/O.
func PatchBytes(original, replacement []byte, marker string, opts PatchOptions) ([]byte, error) {
	if err := ValidateMarker(marker); err != nil {
		return nil, err
	}
	fence, err := ParseFence(string(opts.Fence))
	if err != nil {
		return nil, err
	}
	begin := fmt.Sprintf("<!-- %s begin -->", marker)
	end := fmt.Sprintf("<!-- %s end -->", marker)

	hasCRLF, hasBareLF := detectEOLStyle(original)

	bi, ei, err := markerLineIndices(original, begin, end)
	if err != nil {
		return nil, err
	}

	normalized := NormalizeEOL(replacement, EOLLF)
	textStr := string(bytes.TrimRight(normalized, "\n"))
	middle := buildMiddleLines(textStr, fence)

	var b strings.Builder
	sc := newLineScanner(bytes.NewReader(original))
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
			if len(middle) > 0 {
				b.WriteString(strings.Join(middle, "\n"))
				b.WriteByte('\n')
			}
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
	original, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	content, err := PatchBytes(original, out, marker, opts)
	if err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	return atomicWriteFile(path, content)
}

func buildMiddleLines(textStr string, fence Fence) []string {
	var textLines []string
	if textStr != "" {
		textLines = strings.Split(textStr, "\n")
	}
	if fence == FenceNone {
		return textLines
	}
	lang := string(fence)
	if lang == "" || fence == FenceText {
		lang = string(FenceText)
	}
	f := fenceForContent(textStr)
	openFence := f + lang
	closeFence := f
	middle := make([]string, 0, len(textLines)+2)
	middle = append(middle, openFence)
	middle = append(middle, textLines...)
	middle = append(middle, closeFence)
	return middle
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

func markerLineIndices(data []byte, begin, end string) (bi, ei int, err error) {
	bi, ei = -1, -1
	sc := newLineScanner(bytes.NewReader(data))
	n := 0
	beginBytes := []byte(begin)
	endBytes := []byte(end)
	for sc.Scan() {
		line := sc.Bytes()
		trimmed := bytes.TrimSpace(line)
		switch {
		case bytes.Equal(trimmed, beginBytes):
			if bi >= 0 {
				if ei >= 0 {
					return -1, -1, fmt.Errorf("%w: duplicate marker block %q", ErrDuplicateMarker, begin)
				}
				return -1, -1, fmt.Errorf("%w: duplicate begin marker %q", ErrDuplicateMarker, begin)
			}
			bi = n
		case bytes.Equal(trimmed, endBytes):
			if bi < 0 {
				return -1, -1, fmt.Errorf("%w or invalid order: %q … %q", ErrMarkerNotFound, begin, end)
			}
			if ei >= 0 {
				return -1, -1, fmt.Errorf("%w: duplicate end marker %q", ErrDuplicateMarker, end)
			}
			ei = n
		}
		n++
	}
	if scanErr := sc.Err(); scanErr != nil {
		return -1, -1, scanErr
	}
	if bi < 0 || ei < 0 || ei <= bi {
		return -1, -1, fmt.Errorf("%w or invalid order: %q … %q", ErrMarkerNotFound, begin, end)
	}
	return bi, ei, nil
}

func atomicWriteFile(path string, data []byte) error {
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		path = resolved
	}
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
