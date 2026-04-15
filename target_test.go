package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPatchTargetFile_EOLHandling(t *testing.T) {

	tmpFile := filepath.Join(t.TempDir(), "target_test.md")

	tests := []struct {
		name string
		base []byte
		out  []byte
		eol  eolMode
		want []byte
	}{
		{
			name: "none preserves LF original style",
			base: []byte("a\n<!-- T begin -->\n<!-- T end -->\nb\n"),
			out:  []byte("1\r\n2\n"), // original is LF; normalize out to LF
			eol:  eolNone,
			want: []byte("a\n<!-- T begin -->\n```text\n1\n2\n```\n<!-- T end -->\nb\n"),
		},
		{
			name: "none preserves CRLF original fully",
			base: []byte("a\r\n<!-- T begin -->\r\n<!-- T end -->\r\nb\r\n"),
			out:  []byte("1\n"),
			eol:  eolNone,
			want: []byte("a\r\n<!-- T begin -->\r\n```text\r\n1\r\n```\r\n<!-- T end -->\r\nb\r\n"),
		},
		{
			name: "lf forces LF everywhere",
			base: []byte("a\r\n<!-- T begin -->\n<!-- T end -->\r\nb"),
			out:  []byte("1\r\n2\r"), // trailing \r gets stripped
			eol:  eolLF,
			want: []byte("a\n<!-- T begin -->\n```text\n1\n2\n```\n<!-- T end -->\nb\n"),
		},
		{
			name: "crlf forces CRLF everywhere",
			base: []byte("a\n<!-- T begin -->\n<!-- T end -->\n"),
			out:  []byte("1\n2\r\n"),
			eol:  eolCRLF,
			want: []byte("a\r\n<!-- T begin -->\r\n```text\r\n1\r\n2\r\n```\r\n<!-- T end -->\r\n"),
		},
		{
			name: "none defaults to LF for mixed-EOL original",
			base: []byte("a\n<!-- T begin -->\r\n<!-- T end -->\nb"),
			out:  []byte("1\n"),
			eol:  eolNone,
			want: []byte("a\n<!-- T begin -->\n```text\n1\n```\n<!-- T end -->\nb\n"),
		},
		{
			name: "none preserves CRLF even with internal bare CR",
			base: []byte("a\r\n<!-- T begin -->\r\n<!-- T end -->\r\nb\r\n"),
			out:  []byte("1\r2\r\n"),
			eol:  eolNone,
			want: []byte("a\r\n<!-- T begin -->\r\n```text\r\n1\r2\r\n```\r\n<!-- T end -->\r\nb\r\n"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := os.WriteFile(tmpFile, tt.base, 0o644); err != nil {
				t.Fatalf("WriteFile error: %v", err)
			}
			if err := patchTargetFile(tmpFile, tt.out, "T", tt.eol); err != nil {
				t.Fatalf("patchTargetFile error: %v", err)
			}
			got, err := os.ReadFile(tmpFile)
			if err != nil {
				t.Fatalf("ReadFile error: %v", err)
			}
			if !bytes.Equal(got, tt.want) {
				t.Errorf("patchTargetFile()\ngot : %q\nwant: %q", got, tt.want)
			}
		})
	}
}

func TestPatchTargetFile_LongLine(t *testing.T) {
	tmpFile := filepath.Join(t.TempDir(), "longline.md")
	// Longer than bufio.Scanner's default 64KiB max token; must not return ErrTooLong.
	long := strings.Repeat("x", 70000)
	base := "<!-- T begin -->\n" + long + "\n<!-- T end -->\n"
	if err := os.WriteFile(tmpFile, []byte(base), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := patchTargetFile(tmpFile, []byte("ok\n"), "T", eolNone); err != nil {
		t.Fatalf("patchTargetFile: %v", err)
	}
	got, err := os.ReadFile(tmpFile)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(got, []byte("ok")) {
		t.Fatalf("expected patched output to contain ok, got len %d", len(got))
	}
}
