// Command ptyhelp runs subprocesses in two modes:
//
//	ptyhelp run   — fixed-size pseudo-terminal (stable line wrapping for CLIs such as go-flags).
//	ptyhelp patch — capture stdout and replace a <!-- marker begin/end --> block in a Markdown file;
//	                optional -pty for the same PTY behavior as run; default is plain pipes (inherit env).
//
// There is no default subprocess; you must pass a command (see subcommand help).
package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/apstndb/ptyhelp/internal/textutil"
)

func main() {
	if len(os.Args) < 2 {
		usage(os.Stderr)
		os.Exit(1)
	}
	switch os.Args[1] {
	case "run":
		cmdRun(os.Args[2:])
	case "patch":
		cmdPatch(os.Args[2:])
	case "help", "-h", "--help":
		usage(os.Stdout)
		os.Exit(0)
	default:
		fmt.Fprintf(os.Stderr, "ptyhelp: unknown subcommand %q\n\n", os.Args[1])
		usage(os.Stderr)
		os.Exit(1)
	}
}

func usage(w io.Writer) {
	fmt.Fprintf(w, `ptyhelp — run a command in a PTY, or patch a Markdown marker from command stdout.

usage:
  ptyhelp run   [flags] command args...
  ptyhelp patch [flags] command args...
  ptyhelp help

Run from the module root when using go run . --help.

`)
}

// captureExitCode maps a capture error to a subprocess exit code, or prints a
// fatal error and exits when the failure is not an exec.ExitError.
func captureExitCode(prefix string, err error) int {
	if err == nil {
		return 0
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return exitErr.ExitCode()
	}
	fmt.Fprintf(os.Stderr, "%s: %v\n", prefix, err)
	os.Exit(1)
	return 0
}

func writeChildStderr(prefix string, stderr []byte) {
	if len(stderr) == 0 {
		return
	}
	if _, err := os.Stderr.Write(stderr); err != nil {
		fmt.Fprintf(os.Stderr, "%s: %v\n", prefix, err)
		os.Exit(1)
	}
}

func writeRunStdout(prefix string, stdout []byte, outPath string) {
	if outPath != "" {
		if err := os.WriteFile(outPath, stdout, 0o644); err != nil {
			fmt.Fprintf(os.Stderr, "%s: write %s: %v\n", prefix, outPath, err)
			os.Exit(1)
		}
		return
	}
	if _, err := os.Stdout.Write(stdout); err != nil {
		fmt.Fprintf(os.Stderr, "%s: %v\n", prefix, err)
		os.Exit(1)
	}
}

func writeOptionalStdoutFile(prefix string, stdout []byte, outPath string) {
	if outPath == "" {
		return
	}
	if err := os.WriteFile(outPath, stdout, 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "%s: write %s: %v\n", prefix, outPath, err)
		os.Exit(1)
	}
}

// subcommandHelpOnly handles -h/--help before flag.Parse. Only arguments before
// the first "--" are considered so ptyhelp run -- ... does not treat the child's
// flags as ptyhelp's.
func subcommandHelpOnly(fs *flag.FlagSet, args []string) {
	bound := len(args)
	for i, a := range args {
		if a == "--" {
			bound = i
			break
		}
	}
	for _, a := range args[:bound] {
		switch a {
		case "-h", "--help", "-help":
			fs.Usage()
			os.Exit(0)
		}
	}
}

func cmdRun(args []string) {
	fs := flag.NewFlagSet("run", flag.ExitOnError)
	cols := fs.Uint("cols", 256, "PTY width in columns")
	rows := fs.Uint("rows", 40, "PTY height in rows")
	outPath := fs.String("o", "", "write child stdout to this file instead of printing it")
	normEOL := fs.String("normalize-eol", "none", "normalize line endings: none, lf, crlf")
	fs.Usage = func() {
		fmt.Fprintf(fs.Output(), "usage: ptyhelp run [flags] command args...\n\n")
		fs.PrintDefaults()
		fmt.Fprintf(fs.Output(), `
Runs the command in a pseudo-terminal with the given size (Unix: stdout and stderr stay separate).

`)
	}
	subcommandHelpOnly(fs, args)
	fs.Parse(args)

	eol, err := textutil.ParseEOLMode(*normEOL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ptyhelp run: %v\n", err)
		os.Exit(1)
	}

	argv := fs.Args()
	if len(argv) == 0 {
		fmt.Fprintf(os.Stderr, "ptyhelp run: missing command (example: ptyhelp run -- go run . --help)\n")
		os.Exit(1)
	}

	stdout, stderr, err := capturePTY(*cols, *rows, argv)
	exitCode := captureExitCode("ptyhelp run", err)

	stdout = textutil.NormalizeEOL(stdout, eol)

	writeRunStdout("ptyhelp run", stdout, *outPath)
	writeChildStderr("ptyhelp run", stderr)

	if exitCode != 0 {
		os.Exit(exitCode)
	}
}

func cmdPatch(args []string) {
	fs := flag.NewFlagSet("patch", flag.ExitOnError)
	cols := fs.Uint("cols", 256, "PTY width (setting this flag implies PTY capture; cannot combine with -pty=false)")
	rows := fs.Uint("rows", 40, "PTY height (setting this flag implies PTY capture; cannot combine with -pty=false)")
	ptyFlag := fs.Bool("pty", false, "run in a pseudo-terminal (redundant if -cols or -rows is set)")
	file := fs.String("file", "", "markdown file to patch (required)")
	marker := fs.String("marker", "cli-output", "HTML comment name between <!-- NAME begin --> and <!-- NAME end -->")
	outPath := fs.String("o", "", "also write child stdout to this file")
	normEOL := fs.String("normalize-eol", "none", "normalize line endings: none, lf, crlf")
	fs.Usage = func() {
		fmt.Fprintf(fs.Output(), "usage: ptyhelp patch [flags] command args...\n\n")
		fs.PrintDefaults()
		fmt.Fprintf(fs.Output(), `
Replaces the lines between <!-- MARKER begin --> and <!-- MARKER end --> with a fenced text block
built from the command's stdout. Child stderr is copied to stderr when separated (e.g. on Unix or non-PTY mode).
Note: in PTY mode on non-Unix platforms, stderr is typically merged into stdout.

`)
	}
	subcommandHelpOnly(fs, args)
	fs.Parse(args)

	eol, err := textutil.ParseEOLMode(*normEOL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ptyhelp patch: %v\n", err)
		os.Exit(1)
	}

	var colsSet, rowsSet, ptyVisited bool
	fs.Visit(func(f *flag.Flag) {
		switch f.Name {
		case "cols":
			colsSet = true
		case "rows":
			rowsSet = true
		case "pty":
			ptyVisited = true
		}
	})

	if ptyVisited && !*ptyFlag && (colsSet || rowsSet) {
		fmt.Fprintf(os.Stderr, "ptyhelp patch: -cols and -rows require a PTY; remove -pty=false\n")
		os.Exit(1)
	}

	var runInPTY bool
	if ptyVisited && !*ptyFlag {
		runInPTY = false
	} else {
		runInPTY = *ptyFlag || colsSet || rowsSet
	}

	if *file == "" {
		fmt.Fprintf(os.Stderr, "ptyhelp patch: -file is required\n")
		os.Exit(1)
	}

	argv := fs.Args()
	if len(argv) == 0 {
		fmt.Fprintf(os.Stderr, "ptyhelp patch: missing command (example: ptyhelp patch -file README.md -marker x -pty -- go run . --help)\n")
		os.Exit(1)
	}

	var stdout, stderr []byte
	if runInPTY {
		stdout, stderr, err = capturePTY(*cols, *rows, argv)
	} else {
		stdout, stderr, err = capturePlain(argv)
	}
	exitCode := captureExitCode("ptyhelp patch", err)

	stdout = textutil.NormalizeEOL(stdout, eol)

	tp, err := filepath.Abs(*file)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ptyhelp patch: %v\n", err)
		os.Exit(1)
	}
	if err := textutil.PatchMarkdownFile(tp, stdout, *marker, eol); err != nil {
		fmt.Fprintf(os.Stderr, "ptyhelp patch: %v\n", err)
		os.Exit(1)
	}

	writeOptionalStdoutFile("ptyhelp patch", stdout, *outPath)
	writeChildStderr("ptyhelp patch", stderr)

	if exitCode != 0 {
		os.Exit(exitCode)
	}
}
