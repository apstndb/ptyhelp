package ptycapture_test

import (
	"bytes"
	"context"
	"fmt"
	"time"

	"github.com/apstndb/ptyhelp/ptycapture"
)

func ExampleCapturePlain() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	stdout, stderr, err := ptycapture.CapturePlain(ctx, ptycapture.Options{
		KillAfter: time.Second,
	}, []string{"go", "version"})
	fmt.Println(err == nil)
	fmt.Println(bytes.HasPrefix(stdout, []byte("go version ")))
	fmt.Println(len(stderr) == 0)
	// Output:
	// true
	// true
	// true
}
