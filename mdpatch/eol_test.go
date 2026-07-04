package mdpatch

import (
	"bytes"
	"testing"
)

func TestParseEOLMode(t *testing.T) {
	tests := []struct {
		input   string
		want    EOLMode
		wantErr bool
	}{
		{"none", EOLNone, false},
		{"lf", EOLLF, false},
		{"crlf", EOLCRLF, false},
		{"", EOLNone, true},
		{"LF", EOLNone, true},
		{"CRLF", EOLNone, true},
		{"auto", EOLNone, true},
	}
	for _, tt := range tests {
		got, err := ParseEOLMode(tt.input)
		if (err != nil) != tt.wantErr {
			t.Errorf("ParseEOLMode(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			continue
		}
		if got != tt.want {
			t.Errorf("ParseEOLMode(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

func TestNormalizeEOL(t *testing.T) {
	tests := []struct {
		name  string
		input []byte
		mode  EOLMode
		want  []byte
	}{
		{"none preserves LF", []byte("a\nb\n"), EOLNone, []byte("a\nb\n")},
		{"none preserves CRLF", []byte("a\r\nb\r\n"), EOLNone, []byte("a\r\nb\r\n")},
		{"none preserves mixed", []byte("a\r\nb\nc\r\n"), EOLNone, []byte("a\r\nb\nc\r\n")},

		{"lf from LF", []byte("a\nb\n"), EOLLF, []byte("a\nb\n")},
		{"lf from CRLF", []byte("a\r\nb\r\n"), EOLLF, []byte("a\nb\n")},
		{"lf from mixed", []byte("a\r\nb\nc\r\n"), EOLLF, []byte("a\nb\nc\n")},

		{"crlf from LF", []byte("a\nb\n"), EOLCRLF, []byte("a\r\nb\r\n")},
		{"crlf from CRLF", []byte("a\r\nb\r\n"), EOLCRLF, []byte("a\r\nb\r\n")},
		{"crlf from mixed", []byte("a\r\nb\nc\r\n"), EOLCRLF, []byte("a\r\nb\r\nc\r\n")},

		{"lf empty", []byte{}, EOLLF, []byte{}},
		{"crlf empty", []byte{}, EOLCRLF, []byte{}},
		{"none empty", []byte{}, EOLNone, []byte{}},

		{"lf no newlines", []byte("hello"), EOLLF, []byte("hello")},
		{"crlf no newlines", []byte("hello"), EOLCRLF, []byte("hello")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NormalizeEOL(tt.input, tt.mode)
			if !bytes.Equal(got, tt.want) {
				t.Errorf("NormalizeEOL(%q, %v) = %q, want %q", tt.input, tt.mode, got, tt.want)
			}
		})
	}
}
