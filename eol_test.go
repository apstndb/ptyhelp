package main

import (
	"bytes"
	"testing"

	"github.com/apstndb/ptyhelp/internal/textutil"
)

func TestParseEOLMode(t *testing.T) {
	tests := []struct {
		input   string
		want    textutil.EOLMode
		wantErr bool
	}{
		{"none", textutil.EOLNone, false},
		{"lf", textutil.EOLLF, false},
		{"crlf", textutil.EOLCRLF, false},
		{"", textutil.EOLNone, true},
		{"LF", textutil.EOLNone, true},
		{"CRLF", textutil.EOLNone, true},
		{"auto", textutil.EOLNone, true},
	}
	for _, tt := range tests {
		got, err := textutil.ParseEOLMode(tt.input)
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
		mode  textutil.EOLMode
		want  []byte
	}{
		{"none preserves LF", []byte("a\nb\n"), textutil.EOLNone, []byte("a\nb\n")},
		{"none preserves CRLF", []byte("a\r\nb\r\n"), textutil.EOLNone, []byte("a\r\nb\r\n")},
		{"none preserves mixed", []byte("a\r\nb\nc\r\n"), textutil.EOLNone, []byte("a\r\nb\nc\r\n")},
		{"none preserves bare CR", []byte("1\r2\r\n"), textutil.EOLNone, []byte("1\r2\r\n")},

		{"lf from LF", []byte("a\nb\n"), textutil.EOLLF, []byte("a\nb\n")},
		{"lf from CRLF", []byte("a\r\nb\r\n"), textutil.EOLLF, []byte("a\nb\n")},
		{"lf from mixed", []byte("a\r\nb\nc\r\n"), textutil.EOLLF, []byte("a\nb\nc\n")},
		{"lf preserves bare CR", []byte("1\r2\r\n"), textutil.EOLLF, []byte("1\r2\n")},

		{"crlf from LF", []byte("a\nb\n"), textutil.EOLCRLF, []byte("a\r\nb\r\n")},
		{"crlf from CRLF", []byte("a\r\nb\r\n"), textutil.EOLCRLF, []byte("a\r\nb\r\n")},
		{"crlf from mixed", []byte("a\r\nb\nc\r\n"), textutil.EOLCRLF, []byte("a\r\nb\r\nc\r\n")},
		{"crlf preserves bare CR", []byte("1\r2\n"), textutil.EOLCRLF, []byte("1\r2\r\n")},

		{"lf empty", []byte{}, textutil.EOLLF, []byte{}},
		{"crlf empty", []byte{}, textutil.EOLCRLF, []byte{}},
		{"none empty", []byte{}, textutil.EOLNone, []byte{}},

		{"lf no newlines", []byte("hello"), textutil.EOLLF, []byte("hello")},
		{"crlf no newlines", []byte("hello"), textutil.EOLCRLF, []byte("hello")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := textutil.NormalizeEOL(tt.input, tt.mode)
			if !bytes.Equal(got, tt.want) {
				t.Errorf("NormalizeEOL(%q, %v) = %q, want %q", tt.input, tt.mode, got, tt.want)
			}
		})
	}
}
