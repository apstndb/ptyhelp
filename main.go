// Command ptyhelp runs an arbitrary command inside a PTY with a fixed width so line-oriented
// CLIs (e.g. go-flags --help) wrap predictably; see https://github.com/jessevdk/go-flags/issues/423.
// It uses github.com/aymanbagabas/go-pty (ConPTY on Windows, POSIX pty elsewhere).
//
// Default command is "go run . --help" in the current working directory; pass
// "ptyhelp [flags] -- cmd args..." to override. Run ptyhelp from the module root (or cd there first).
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
)

func main() {
	cols := flag.Uint("cols", 256, "PTY width in columns")
	rows := flag.Uint("rows", 40, "PTY height in rows")
	outPath := flag.String("o", "", "write raw command output to this file")
	targetFile := flag.String("target-file", "", "replace <!-- <marker> begin/end --> block in this file (see -marker)")
	marker := flag.String("marker", "cli-output", "HTML comment name between <!-- NAME begin --> and <!-- NAME end -->")
	bin := flag.String("binary", "", "shorthand: run BINARY --help (ignored if a command is given after --)")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "usage: ptyhelp [flags] [-- command args...]\n\n")
		fmt.Fprintf(os.Stderr, "If no command is given after --, uses -binary with --help, or else \"go run . --help\".\n\n")
		flag.PrintDefaults()
	}
	flag.Parse()

	argv := flag.Args()
	if len(argv) == 0 && *bin != "" {
		argv = []string{*bin, "--help"}
	}
	if len(argv) == 0 {
		argv = []string{"go", "run", ".", "--help"}
	}

	b, err := captureCommand(*cols, *rows, argv)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ptyhelp: %v\n", err)
		os.Exit(1)
	}

	if *targetFile != "" {
		tp, err := filepath.Abs(*targetFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "ptyhelp: %v\n", err)
			os.Exit(1)
		}
		if err := patchTargetFile(tp, b, *marker); err != nil {
			fmt.Fprintf(os.Stderr, "ptyhelp: target-file: %v\n", err)
			os.Exit(1)
		}
	}
	if *outPath != "" {
		if err := os.WriteFile(*outPath, b, 0o644); err != nil {
			fmt.Fprintf(os.Stderr, "ptyhelp: write %s: %v\n", *outPath, err)
			os.Exit(1)
		}
	}
	if *targetFile == "" && *outPath == "" {
		if _, err := os.Stdout.Write(b); err != nil {
			fmt.Fprintf(os.Stderr, "ptyhelp: %v\n", err)
			os.Exit(1)
		}
	}
}
