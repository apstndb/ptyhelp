package mdpatch_test

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/apstndb/ptyhelp/mdpatch"
)

func ExamplePatchBytes() {
	original := []byte("<!-- help begin -->\nold\n<!-- help end -->\n")
	patched, err := mdpatch.PatchBytes(original, []byte("new\n"), "help", mdpatch.PatchOptions{
		Fence: mdpatch.FenceNone,
	})
	if err != nil {
		panic(err)
	}
	fmt.Print(string(patched))
	// Output:
	// <!-- help begin -->
	// new
	// <!-- help end -->
}

func ExamplePatchMarkdownFile() {
	dir, err := os.MkdirTemp("", "mdpatch-example-")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(dir)

	path := filepath.Join(dir, "README.md")
	if err := os.WriteFile(path, []byte("<!-- help begin -->\nold\n<!-- help end -->\n"), 0o644); err != nil {
		panic(err)
	}
	if err := mdpatch.PatchMarkdownFile(path, []byte("usage\n"), "help", mdpatch.PatchOptions{}); err != nil {
		panic(err)
	}
	patched, err := os.ReadFile(path)
	if err != nil {
		panic(err)
	}
	fmt.Print(string(patched))
	// Output:
	// <!-- help begin -->
	// ```text
	// usage
	// ```
	// <!-- help end -->
}

func ExampleNormalizeEOL() {
	fmt.Printf("%q\n", mdpatch.NormalizeEOL([]byte("one\r\ntwo\n"), mdpatch.EOLCRLF))
	// Output: "one\r\ntwo\r\n"
}
