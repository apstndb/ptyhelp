package ptycapture

import (
	"strings"
	"testing"
	"time"
)

func TestApplyStderrMode(t *testing.T) {
	stdout, stderr := ApplyStderrMode([]byte("out"), []byte("err"), StderrMerge)
	if string(stdout) != "outerr" || len(stderr) != 0 {
		t.Fatalf("merge: stdout=%q stderr=%q", stdout, stderr)
	}
	stdout, stderr = ApplyStderrMode([]byte("out"), []byte("err"), StderrDiscard)
	if string(stdout) != "out" || len(stderr) != 0 {
		t.Fatalf("discard: stdout=%q stderr=%q", stdout, stderr)
	}
	stdout, stderr = ApplyStderrMode([]byte("out"), []byte("err"), StderrSeparate)
	if string(stdout) != "out" || string(stderr) != "err" {
		t.Fatalf("separate: stdout=%q stderr=%q", stdout, stderr)
	}
}

func TestParseStderrMode(t *testing.T) {
	mode, err := ParseStderrMode("merge")
	if err != nil || mode != StderrMerge {
		t.Fatalf("merge: %v %v", mode, err)
	}
	if _, err := ParseStderrMode("bogus"); err == nil {
		t.Fatal("expected error")
	}
}

func TestCapturePlain_MergeStderrIntegration(t *testing.T) {
	stdout, stderr, err := CapturePlain(Options{}, []string{"/bin/sh", "-c", "printf out; printf err 1>&2"})
	if err != nil {
		t.Fatal(err)
	}
	merged, _ := ApplyStderrMode(stdout, stderr, StderrMerge)
	if string(merged) != "outerr" {
		t.Fatalf("got %q", merged)
	}
}

func TestCapturePlain_MaxOutputBytes(t *testing.T) {
	_, _, err := CapturePlain(Options{MaxOutputBytes: 4}, []string{"/bin/sh", "-c", "printf hello"})
	if err == nil || !strings.Contains(err.Error(), "exceeded") {
		t.Fatalf("expected max bytes error, got %v", err)
	}
}

func TestCapturePlain_Timeout(t *testing.T) {
	_, _, err := CapturePlain(Options{Timeout: 50 * time.Millisecond}, []string{"/bin/sh", "-c", "sleep 5"})
	if err == nil {
		t.Fatal("expected timeout error")
	}
}
