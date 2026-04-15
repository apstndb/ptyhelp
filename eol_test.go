package main

import (
	"bytes"
	"testing"
)

func TestParseEOLMode(t *testing.T) {
	tests := []struct {
		input   string
		want    eolMode
		wantErr bool
	}{
		{"none", eolNone, false},
		{"lf", eolLF, false},
		{"crlf", eolCRLF, false},
		{"", eolNone, true},
		{"LF", eolNone, true},
		{"CRLF", eolNone, true},
		{"auto", eolNone, true},
	}
	for _, tt := range tests {
		got, err := parseEOLMode(tt.input)
		if (err != nil) != tt.wantErr {
			t.Errorf("parseEOLMode(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			continue
		}
		if got != tt.want {
			t.Errorf("parseEOLMode(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

func TestNormalizeEOL(t *testing.T) {
	tests := []struct {
		name  string
		input []byte
		mode  eolMode
		want  []byte
	}{
		{"none preserves LF", []byte("a\nb\n"), eolNone, []byte("a\nb\n")},
		{"none preserves CRLF", []byte("a\r\nb\r\n"), eolNone, []byte("a\r\nb\r\n")},
		{"none preserves mixed", []byte("a\r\nb\nc\r\n"), eolNone, []byte("a\r\nb\nc\r\n")},

		{"lf from LF", []byte("a\nb\n"), eolLF, []byte("a\nb\n")},
		{"lf from CRLF", []byte("a\r\nb\r\n"), eolLF, []byte("a\nb\n")},
		{"lf from mixed", []byte("a\r\nb\nc\r\n"), eolLF, []byte("a\nb\nc\n")},

		{"crlf from LF", []byte("a\nb\n"), eolCRLF, []byte("a\r\nb\r\n")},
		{"crlf from CRLF", []byte("a\r\nb\r\n"), eolCRLF, []byte("a\r\nb\r\n")},
		{"crlf from mixed", []byte("a\r\nb\nc\r\n"), eolCRLF, []byte("a\r\nb\r\nc\r\n")},

		{"lf empty", []byte{}, eolLF, []byte{}},
		{"crlf empty", []byte{}, eolCRLF, []byte{}},
		{"none empty", []byte{}, eolNone, []byte{}},

		{"lf no newlines", []byte("hello"), eolLF, []byte("hello")},
		{"crlf no newlines", []byte("hello"), eolCRLF, []byte("hello")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := normalizeEOL(tt.input, tt.mode)
			if !bytes.Equal(got, tt.want) {
				t.Errorf("normalizeEOL(%q, %v) = %q, want %q", tt.input, tt.mode, got, tt.want)
			}
		})
	}
}
