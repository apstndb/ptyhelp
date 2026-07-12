package mdpatch

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func defaultOpts(eol EOLMode) PatchOptions {
	return PatchOptions{EOL: eol, Fence: FenceText}
}

func TestPatchMarkdownFile_EOLHandling(t *testing.T) {
	tmpFile := filepath.Join(t.TempDir(), "target_test.md")

	tests := []struct {
		name string
		base []byte
		out  []byte
		eol  EOLMode
		want []byte
	}{
		{
			name: "none preserves LF original style",
			base: []byte("a\n<!-- T begin -->\n<!-- T end -->\nb\n"),
			out:  []byte("1\r\n2\n"),
			eol:  EOLNone,
			want: []byte("a\n<!-- T begin -->\n```text\n1\n2\n```\n<!-- T end -->\nb\n"),
		},
		{
			name: "none preserves CRLF original fully",
			base: []byte("a\r\n<!-- T begin -->\r\n<!-- T end -->\r\nb\r\n"),
			out:  []byte("1\n"),
			eol:  EOLNone,
			want: []byte("a\r\n<!-- T begin -->\r\n```text\r\n1\r\n```\r\n<!-- T end -->\r\nb\r\n"),
		},
		{
			name: "lf forces LF everywhere",
			base: []byte("a\r\n<!-- T begin -->\n<!-- T end -->\r\nb"),
			out:  []byte("1\r\n2\r"),
			eol:  EOLLF,
			want: []byte("a\n<!-- T begin -->\n```text\n1\n2\n```\n<!-- T end -->\nb\n"),
		},
		{
			name: "crlf forces CRLF everywhere",
			base: []byte("a\n<!-- T begin -->\n<!-- T end -->\n"),
			out:  []byte("1\n2\r\n"),
			eol:  EOLCRLF,
			want: []byte("a\r\n<!-- T begin -->\r\n```text\r\n1\r\n2\r\n```\r\n<!-- T end -->\r\n"),
		},
		{
			name: "none defaults to LF for mixed-EOL original",
			base: []byte("a\n<!-- T begin -->\r\n<!-- T end -->\nb"),
			out:  []byte("1\n"),
			eol:  EOLNone,
			want: []byte("a\n<!-- T begin -->\n```text\n1\n```\n<!-- T end -->\nb\n"),
		},
		{
			name: "none preserves CRLF even with internal bare CR",
			base: []byte("a\r\n<!-- T begin -->\r\n<!-- T end -->\r\nb\r\n"),
			out:  []byte("1\r2\r\n"),
			eol:  EOLNone,
			want: []byte("a\r\n<!-- T begin -->\r\n```text\r\n1\r2\r\n```\r\n<!-- T end -->\r\nb\r\n"),
		},
		{
			name: "preserves leading indent",
			base: []byte("a\n<!-- T begin -->\n<!-- T end -->\n"),
			out:  []byte("  indented\n"),
			eol:  EOLNone,
			want: []byte("a\n<!-- T begin -->\n```text\n  indented\n```\n<!-- T end -->\n"),
		},
		{
			name: "preserves trailing spaces on last line",
			base: []byte("a\n<!-- T begin -->\n<!-- T end -->\n"),
			out:  []byte("line  \n"),
			eol:  EOLNone,
			want: []byte("a\n<!-- T begin -->\n```text\nline  \n```\n<!-- T end -->\n"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := os.WriteFile(tmpFile, tt.base, 0o644); err != nil {
				t.Fatalf("WriteFile error: %v", err)
			}
			if err := PatchMarkdownFile(tmpFile, tt.out, "T", defaultOpts(tt.eol)); err != nil {
				t.Fatalf("PatchMarkdownFile error: %v", err)
			}
			got, err := os.ReadFile(tmpFile)
			if err != nil {
				t.Fatalf("ReadFile error: %v", err)
			}
			if !bytes.Equal(got, tt.want) {
				t.Errorf("PatchMarkdownFile()\ngot : %q\nwant: %q", got, tt.want)
			}
		})
	}
}

func TestPatchMarkdownFile_FenceNone(t *testing.T) {
	tmpFile := filepath.Join(t.TempDir(), "raw.md")
	base := "a\n<!-- T begin -->\nold\n<!-- T end -->\nb\n"
	if err := os.WriteFile(tmpFile, []byte(base), 0o644); err != nil {
		t.Fatal(err)
	}
	opts := PatchOptions{EOL: EOLNone, Fence: FenceNone}
	if err := PatchMarkdownFile(tmpFile, []byte("raw line\n"), "T", opts); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(tmpFile)
	if err != nil {
		t.Fatal(err)
	}
	want := "a\n<!-- T begin -->\nraw line\n<!-- T end -->\nb\n"
	if string(got) != want {
		t.Fatalf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestPatchMarkdownFile_CustomFenceLang(t *testing.T) {
	tmpFile := filepath.Join(t.TempDir(), "go.md")
	base := "<!-- T begin -->\n<!-- T end -->\n"
	if err := os.WriteFile(tmpFile, []byte(base), 0o644); err != nil {
		t.Fatal(err)
	}
	opts := PatchOptions{EOL: EOLNone, Fence: Fence("go")}
	if err := PatchMarkdownFile(tmpFile, []byte("package main\n"), "T", opts); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(tmpFile)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(got, []byte("```go\npackage main\n```\n")) {
		t.Fatalf("unexpected content:\n%s", got)
	}
}

func TestPatchMarkdownFile_LongLine(t *testing.T) {
	tmpFile := filepath.Join(t.TempDir(), "longline.md")
	long := strings.Repeat("x", 70000)
	base := "<!-- T begin -->\n" + long + "\n<!-- T end -->\n"
	if err := os.WriteFile(tmpFile, []byte(base), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := PatchMarkdownFile(tmpFile, []byte("ok\n"), "T", defaultOpts(EOLNone)); err != nil {
		t.Fatalf("PatchMarkdownFile: %v", err)
	}
	got, err := os.ReadFile(tmpFile)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(got, []byte("ok")) {
		t.Fatalf("expected patched output to contain ok, got len %d", len(got))
	}
}

func TestPatchMarkdownFileRequiresExactMarkerLines(t *testing.T) {
	tmpFile := filepath.Join(t.TempDir(), "markers.md")
	base := "note <!-- T begin --> inline\n<!-- T begin -->\nold\n<!-- T end -->\n"
	if err := os.WriteFile(tmpFile, []byte(base), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := PatchMarkdownFile(tmpFile, []byte("ok\n"), "T", defaultOpts(EOLNone)); err != nil {
		t.Fatalf("PatchMarkdownFile: %v", err)
	}
	got, err := os.ReadFile(tmpFile)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(got, []byte("```text\nok\n```")) {
		t.Fatalf("expected fenced ok block, got:\n%s", got)
	}
	if bytes.Contains(got, []byte("note <!-- T begin --> inline\n```text")) {
		t.Fatalf("inline marker substring must not start the fenced block")
	}
}

func TestPatchMarkdownFile_FenceCollision(t *testing.T) {
	tmpFile := filepath.Join(t.TempDir(), "fence.md")
	base := "<!-- T begin -->\n<!-- T end -->\n"
	if err := os.WriteFile(tmpFile, []byte(base), 0o644); err != nil {
		t.Fatal(err)
	}
	out := []byte("example:\n```\nnot a fence\n```\n")
	if err := PatchMarkdownFile(tmpFile, out, "T", defaultOpts(EOLNone)); err != nil {
		t.Fatalf("PatchMarkdownFile: %v", err)
	}
	got, err := os.ReadFile(tmpFile)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(got, []byte("````text\nexample:\n```\nnot a fence\n```\n````\n")) {
		t.Fatalf("expected 4-backtick fence, got:\n%s", got)
	}
}

func TestPatchMarkdownFile_MarkerErrors(t *testing.T) {
	tmpDir := t.TempDir()

	tests := []struct {
		name    string
		content string
		marker  string
		wantErr string
	}{
		{
			name:    "end before begin",
			content: "<!-- T end -->\n<!-- T begin -->\n",
			marker:  "T",
			wantErr: "invalid order",
		},
		{
			name:    "missing end",
			content: "<!-- T begin -->\n",
			marker:  "T",
			wantErr: "not found",
		},
		{
			name:    "duplicate begin",
			content: "<!-- T begin -->\n<!-- T begin -->\n<!-- T end -->\n",
			marker:  "T",
			wantErr: "duplicate begin",
		},
		{
			name:    "duplicate end",
			content: "<!-- T begin -->\n<!-- T end -->\n<!-- T end -->\n",
			marker:  "T",
			wantErr: "duplicate end",
		},
		{
			name:    "duplicate block later in file",
			content: "<!-- T begin -->\n<!-- T end -->\n<!-- T begin -->\n<!-- T end -->\n",
			marker:  "T",
			wantErr: "duplicate marker block",
		},
		{
			name:    "invalid marker name",
			content: "<!-- T begin -->\n<!-- T end -->\n",
			marker:  "bad>name",
			wantErr: "invalid marker",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(tmpDir, tt.name+".md")
			if err := os.WriteFile(path, []byte(tt.content), 0o644); err != nil {
				t.Fatal(err)
			}
			err := PatchMarkdownFile(path, []byte("x\n"), tt.marker, defaultOpts(EOLNone))
			if err == nil {
				t.Fatal("expected error")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("error = %v, want substring %q", err, tt.wantErr)
			}
		})
	}
}

func TestPatchBytes_CheckMode(t *testing.T) {
	base := "a\n<!-- T begin -->\n```text\nold\n```\n<!-- T end -->\nb\n"
	current := []byte(base)
	same, err := PatchBytes(current, []byte("old\n"), "T", defaultOpts(EOLNone))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(current, same) {
		t.Fatal("expected no change for matching content")
	}
	diff, err := PatchBytes(current, []byte("new\n"), "T", defaultOpts(EOLNone))
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(current, diff) {
		t.Fatal("expected content to differ for new output")
	}
}

func TestFenceForContent(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"plain", "```"},
		{"has ` one", "```"},
		{"has ``` three", "````"},
		{"````four", "`````"},
	}
	for _, tt := range tests {
		if got := fenceForContent(tt.in); got != tt.want {
			t.Errorf("fenceForContent(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestParseFence(t *testing.T) {
	got, err := ParseFence("none")
	if err != nil || got != FenceNone {
		t.Fatalf("ParseFence(none) = %q, %v", got, err)
	}
	for _, tag := range []string{"go", "c++", "c#", "objective-c"} {
		if got, err := ParseFence(tag); err != nil || got != Fence(tag) {
			t.Fatalf("ParseFence(%q) = %q, %v", tag, got, err)
		}
	}
	if _, err := ParseFence("bad lang"); !errors.Is(err, ErrInvalidFence) {
		t.Fatalf("expected ErrInvalidFence, got %v", err)
	}
}

func TestPatchBytes_ZeroFenceIsText(t *testing.T) {
	original := []byte("<!-- T begin -->\n<!-- T end -->\n")
	got, err := PatchBytes(original, []byte("body\n"), "T", PatchOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(got, []byte("```text\nbody\n```")) {
		t.Fatalf("zero fence did not produce text fence:\n%s", got)
	}
}

func TestPatchBytes_ValidatesFenceOnUse(t *testing.T) {
	original := []byte("<!-- T begin -->\n<!-- T end -->\n")
	_, err := PatchBytes(original, []byte("body\n"), "T", PatchOptions{Fence: Fence("bad lang")})
	if !errors.Is(err, ErrInvalidFence) {
		t.Fatalf("expected ErrInvalidFence, got %v", err)
	}
}

func TestValidateMarker(t *testing.T) {
	if err := ValidateMarker("readme-help"); err != nil {
		t.Fatalf("valid marker: %v", err)
	}
	if err := ValidateMarker("a--b"); !errors.Is(err, ErrInvalidMarker) {
		t.Fatalf("expected ErrInvalidMarker for --, got %v", err)
	}
	if err := ValidateMarker("bad name"); !errors.Is(err, ErrInvalidMarker) {
		t.Fatalf("expected ErrInvalidMarker for space, got %v", err)
	}
}

func TestPatchBytes_ErrorCategories(t *testing.T) {
	tests := []struct {
		name     string
		original string
		want     error
	}{
		{name: "missing", original: "plain\n", want: ErrMarkerNotFound},
		{name: "duplicate", original: "<!-- T begin -->\n<!-- T begin -->\n<!-- T end -->\n", want: ErrDuplicateMarker},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := PatchBytes([]byte(tt.original), []byte("body\n"), "T", PatchOptions{})
			if !errors.Is(err, tt.want) {
				t.Fatalf("error = %v, want category %v", err, tt.want)
			}
		})
	}
}
