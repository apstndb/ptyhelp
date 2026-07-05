package ptycapture

import (
	"testing"
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

