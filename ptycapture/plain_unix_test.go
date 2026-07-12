//go:build unix

package ptycapture

import (
	"os"
	"path/filepath"
	"strings"
	"syscall"
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

func TestSafeProcessGroup(t *testing.T) {
	selfPGID, err := syscall.Getpgid(0)
	if err != nil {
		t.Fatal(err)
	}
	for _, pgid := range []int{0, 1, selfPGID} {
		if safeProcessGroup(pgid) {
			t.Fatalf("safeProcessGroup(%d) = true, want false", pgid)
		}
	}
	if !safeProcessGroup(selfPGID + 1) {
		t.Fatalf("safeProcessGroup(%d) = false, want true", selfPGID+1)
	}
}

func TestCapture_KillAfterStopsDescendantsAfterGrace(t *testing.T) {
	captureModes := []struct {
		name    string
		capture func(Options, []string) ([]byte, []byte, error)
	}{
		{name: "plain", capture: CapturePlain},
		{name: "pty", capture: CapturePTY},
	}
	parentBehaviors := []struct {
		name   string
		script string
	}{
		{
			name:   "parent ignores term",
			script: `trap '' TERM; (trap '' TERM HUP; while :; do printf x >> "$1"; sleep 0.02; done) & wait`,
		},
		{
			name:   "parent exits on term",
			script: `(trap '' TERM HUP; while :; do printf x >> "$1"; sleep 0.02; done) & trap 'exit 0' TERM; while :; do :; done`,
		},
	}

	for _, mode := range captureModes {
		for _, behavior := range parentBehaviors {
			t.Run(mode.name+"/"+behavior.name, func(t *testing.T) {
				heartbeat := filepath.Join(t.TempDir(), "heartbeat")
				opts := Options{
					Cols:      80,
					Rows:      24,
					Timeout:   50 * time.Millisecond,
					KillAfter: 250 * time.Millisecond,
				}

				started := time.Now()
				_, _, err := mode.capture(opts, []string{"/bin/sh", "-c", behavior.script, "sh", heartbeat})
				elapsed := time.Since(started)
				if err == nil {
					t.Fatal("expected timeout error")
				}
				if elapsed < opts.KillAfter {
					t.Fatalf("capture returned after %s, before kill-after grace %s", elapsed, opts.KillAfter)
				}

				before, err := os.Stat(heartbeat)
				if err != nil {
					t.Fatal(err)
				}
				time.Sleep(150 * time.Millisecond)
				after, err := os.Stat(heartbeat)
				if err != nil {
					t.Fatal(err)
				}
				if after.Size() != before.Size() {
					t.Fatalf("descendant kept running: heartbeat grew from %d to %d bytes", before.Size(), after.Size())
				}
			})
		}
	}
}

func TestCapturePlain_DrainsLargeOutput(t *testing.T) {
	const size = 256 << 10
	for i := range 100 {
		stdout, _, err := CapturePlain(
			Options{Timeout: 5 * time.Second},
			[]string{"/bin/sh", "-c", "head -c 262144 /dev/zero"},
		)
		if err != nil {
			t.Fatalf("iteration %d: %v", i, err)
		}
		if len(stdout) != size {
			t.Fatalf("iteration %d: captured %d bytes, want %d", i, len(stdout), size)
		}
	}
}
