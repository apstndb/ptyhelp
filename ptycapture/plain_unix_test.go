//go:build unix

package ptycapture

import (
	"strings"
	"testing"
	"time"
)

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

func isMaxOutputErr(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "exceeded")
}

func TestCapturePlain_MaxOutputBytes(t *testing.T) {
	_, _, err := CapturePlain(Options{MaxOutputBytes: 4}, []string{"/bin/sh", "-c", "printf hello"})
	if !isMaxOutputErr(err) {
		t.Fatalf("expected max bytes error, got %v", err)
	}
}

func TestCapturePlain_MaxOutputBytesProlificWriter(t *testing.T) {
	_, _, err := CapturePlain(Options{MaxOutputBytes: 10}, []string{"/bin/sh", "-c", "yes x | head -c 1000000"})
	if !isMaxOutputErr(err) {
		t.Fatalf("expected max bytes error, got %v", err)
	}
}

func TestCapturePTY_MaxOutputBytesProlificWriter(t *testing.T) {
	_, _, err := CapturePTY(Options{Cols: 80, Rows: 24, MaxOutputBytes: 10}, []string{"/bin/sh", "-c", "yes x | head -c 1000000"})
	if !isMaxOutputErr(err) {
		t.Fatalf("expected max bytes error, got %v", err)
	}
}

func TestCapturePlain_Timeout(t *testing.T) {
	_, _, err := CapturePlain(Options{Timeout: 50 * time.Millisecond}, []string{"/bin/sh", "-c", "sleep 5"})
	if err == nil {
		t.Fatal("expected timeout error")
	}
}
