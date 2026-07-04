// Command ptyhelp runs subprocesses in two modes:
//
//	ptyhelp run   — fixed-size pseudo-terminal (stable line wrapping for CLIs such as go-flags).
//	ptyhelp patch — capture stdout and replace a <!-- marker begin/end --> block in a Markdown file;
//	                optional -pty for the same PTY behavior as run; default is plain pipes (inherit env).
//
// There is no default subprocess; you must pass a command (see subcommand help).
package main

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime/debug"
	"time"

	"github.com/apstndb/ptyhelp/mdpatch"
	"github.com/apstndb/ptyhelp/ptycapture"
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
	case "version":
		cmdVersion()
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
	_, _ = fmt.Fprintf(w, `ptyhelp — run a command in a PTY, or patch a Markdown marker from command stdout.

usage:
  ptyhelp run     [flags] command args...
  ptyhelp patch   [flags] command args...
  ptyhelp version
  ptyhelp help

`)
}

func cmdVersion() {
	version := "(devel)"
	if info, ok := debug.ReadBuildInfo(); ok {
		if info.Main.Version != "" && info.Main.Version != "(devel)" {
			version = info.Main.Version
		}
	}
	fmt.Println(version)
}

func captureExitCode(prefix string, err error, stderr []byte) int {
	if err == nil {
		return 0
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return exitCodeFromExitError(exitErr)
	}
	if code, ok := ptycapture.ExitCode(err); ok {
		return code
	}
	writeChildStderr(prefix, stderr)
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

func exitWithError(prefix string, err error) {
	fmt.Fprintf(os.Stderr, "%s: %v\n", prefix, err)
	os.Exit(1)
}

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

func validatePTYSize(cols, rows uint) error {
	if cols == 0 || rows == 0 {
		return fmt.Errorf("cols and rows must be >= 1")
	}
	if cols > 0xffff || rows > 0xffff {
		return fmt.Errorf("cols/rows out of range")
	}
	return nil
}

func parseDurationFlag(name, value string) (time.Duration, error) {
	if value == "" {
		return 0, nil
	}
	d, err := time.ParseDuration(value)
	if err != nil {
		return 0, fmt.Errorf("invalid -%s value %q: %w", name, value, err)
	}
	if d < 0 {
		return 0, fmt.Errorf("invalid -%s value %q: must be >= 0", name, value)
	}
	return d, nil
}

type captureFlags struct {
	stderrMode     ptycapture.StderrMode
	timeout        time.Duration
	killAfter      time.Duration
	maxOutputBytes int64
}

func finalizeCaptureFlags(fs *flag.FlagSet, f *captureFlags, stderrStr *string, combined *bool, timeoutStr, killAfterStr *string, maxOut *int64) {
	mode, err := ptycapture.ParseStderrMode(*stderrStr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %v\n", fs.Name(), err)
		os.Exit(1)
	}
	f.stderrMode = mode
	if *combined {
		f.stderrMode = ptycapture.StderrMerge
	}
	if *timeoutStr != "" {
		d, err := parseDurationFlag("timeout", *timeoutStr)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s: %v\n", fs.Name(), err)
			os.Exit(1)
		}
		f.timeout = d
	}
	if *killAfterStr != "" {
		d, err := parseDurationFlag("kill-after", *killAfterStr)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s: %v\n", fs.Name(), err)
			os.Exit(1)
		}
		f.killAfter = d
	}
	if *maxOut < 0 {
		fmt.Fprintf(os.Stderr, "%s: -max-output-bytes must be >= 0\n", fs.Name())
		os.Exit(1)
	}
	f.maxOutputBytes = *maxOut
}

func (f captureFlags) options(cols, rows uint) ptycapture.Options {
	return ptycapture.Options{
		Cols:           cols,
		Rows:           rows,
		Timeout:        f.timeout,
		KillAfter:      f.killAfter,
		MaxOutputBytes: f.maxOutputBytes,
	}
}

func cmdRun(args []string) {
	fs := flag.NewFlagSet("run", flag.ExitOnError)
	cols := fs.Uint("cols", 256, "PTY width in columns")
	rows := fs.Uint("rows", 40, "PTY height in rows")
	outPath := fs.String("o", "", "write child stdout to this file instead of printing it")
	normEOL := fs.String("normalize-eol", "none", "normalize line endings in captured stdout: none, lf, crlf")
	stderrStr := fs.String("stderr", "separate", "stderr handling: separate, merge, or discard")
	combined := fs.Bool("combined", false, "merge stderr into stdout (same as -stderr=merge)")
	timeoutStr := fs.String("timeout", "", "maximum subprocess runtime (e.g. 30s, 5m)")
	killAfterStr := fs.String("kill-after", "", "grace period after timeout before SIGKILL (e.g. 5s)")
	maxOut := fs.Int64("max-output-bytes", 0, "fail when stdout or stderr exceeds this many bytes (0 = unlimited)")
	fs.Usage = func() {
		_, _ = fmt.Fprintf(fs.Output(), "usage: ptyhelp run [flags] command args...\n\n")
		fs.PrintDefaults()
		_, _ = fmt.Fprintf(fs.Output(), `
Runs the command in a pseudo-terminal with the given size (Unix: stdout and stderr stay separate).

`)
	}
	subcommandHelpOnly(fs, args)
	_ = fs.Parse(args)

	var capFlags captureFlags
	finalizeCaptureFlags(fs, &capFlags, stderrStr, combined, timeoutStr, killAfterStr, maxOut)

	eol, err := mdpatch.ParseEOLMode(*normEOL)
	if err != nil {
		exitWithError("ptyhelp run", err)
	}
	if err := validatePTYSize(*cols, *rows); err != nil {
		exitWithError("ptyhelp run", err)
	}

	argv := fs.Args()
	if len(argv) == 0 {
		fmt.Fprintf(os.Stderr, "ptyhelp run: missing command (example: ptyhelp run -- go run . --help)\n")
		os.Exit(1)
	}

	stdout, stderr, err := ptycapture.CapturePTY(capFlags.options(*cols, *rows), argv)
	exitCode := captureExitCode("ptyhelp run", err, stderr)

	stdout, stderr = ptycapture.ApplyStderrMode(stdout, stderr, capFlags.stderrMode)
	stdout = mdpatch.NormalizeEOL(stdout, eol)

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
	fenceStr := fs.String("fence", "text", "fenced code block language: text, none, or a language tag")
	outPath := fs.String("o", "", "also write child stdout to this file (skipped when the child exits non-zero)")
	normEOL := fs.String("normalize-eol", "none", "normalize line endings in the entire target file: none, lf, crlf")
	check := fs.Bool("check", false, "exit 1 when the target file would change (CI staleness check)")
	dryRun := fs.Bool("dry-run", false, "print the patched file to stdout when it would change, without writing")
	stderrStr := fs.String("stderr", "separate", "stderr handling: separate, merge, or discard")
	combined := fs.Bool("combined", false, "merge stderr into stdout (same as -stderr=merge)")
	timeoutStr := fs.String("timeout", "", "maximum subprocess runtime (e.g. 30s, 5m)")
	killAfterStr := fs.String("kill-after", "", "grace period after timeout before SIGKILL (e.g. 5s)")
	maxOut := fs.Int64("max-output-bytes", 0, "fail when stdout or stderr exceeds this many bytes (0 = unlimited)")
	fs.Usage = func() {
		_, _ = fmt.Fprintf(fs.Output(), "usage: ptyhelp patch [flags] command args...\n\n")
		fs.PrintDefaults()
		_, _ = fmt.Fprintf(fs.Output(), `
Replaces the lines between <!-- MARKER begin --> and <!-- MARKER end --> with captured output.
Use -fence=none for raw Markdown, or command "-" to read patch content from stdin.
Child stderr is copied to stderr when separated (e.g. on Unix or non-PTY mode).
The target file is not modified when the child exits non-zero.
Note: in PTY mode on non-Unix platforms, stderr is typically merged into stdout.

`)
	}
	subcommandHelpOnly(fs, args)
	_ = fs.Parse(args)

	var capFlags captureFlags
	finalizeCaptureFlags(fs, &capFlags, stderrStr, combined, timeoutStr, killAfterStr, maxOut)

	eol, err := mdpatch.ParseEOLMode(*normEOL)
	if err != nil {
		exitWithError("ptyhelp patch", err)
	}
	fence, err := mdpatch.ParseFence(*fenceStr)
	if err != nil {
		exitWithError("ptyhelp patch", err)
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

	if runInPTY {
		if err := validatePTYSize(*cols, *rows); err != nil {
			fmt.Fprintf(os.Stderr, "ptyhelp patch: %v\n", err)
			os.Exit(1)
		}
	}

	var stdout, stderr []byte
	var exitCode int
	if len(argv) == 1 && argv[0] == "-" {
		var readErr error
		stdout, readErr = io.ReadAll(os.Stdin)
		if readErr != nil {
			fmt.Fprintf(os.Stderr, "ptyhelp patch: read stdin: %v\n", readErr)
			os.Exit(1)
		}
	} else {
		opts := capFlags.options(*cols, *rows)
		if runInPTY {
			stdout, stderr, err = ptycapture.CapturePTY(opts, argv)
		} else {
			stdout, stderr, err = ptycapture.CapturePlain(opts, argv)
		}
		exitCode = captureExitCode("ptyhelp patch", err, stderr)
	}

	stdout, stderr = ptycapture.ApplyStderrMode(stdout, stderr, capFlags.stderrMode)
	stdout = mdpatch.NormalizeEOL(stdout, eol)

	patchOpts := mdpatch.PatchOptions{EOL: eol, Fence: fence}

	if exitCode == 0 {
		tp, absErr := filepath.Abs(*file)
		if absErr != nil {
			fmt.Fprintf(os.Stderr, "ptyhelp patch: %v\n", absErr)
			os.Exit(1)
		}

		if *check || *dryRun {
			newContent, buildErr := mdpatch.BuildPatchedContent(tp, stdout, *marker, patchOpts)
			if buildErr != nil {
				fmt.Fprintf(os.Stderr, "ptyhelp patch: %v\n", buildErr)
				os.Exit(1)
			}
			current, readErr := os.ReadFile(tp)
			if readErr != nil {
				fmt.Fprintf(os.Stderr, "ptyhelp patch: %v\n", readErr)
				os.Exit(1)
			}
			if !bytes.Equal(current, newContent) {
				if *dryRun {
					if _, err := os.Stdout.Write(newContent); err != nil {
						fmt.Fprintf(os.Stderr, "ptyhelp patch: %v\n", err)
						os.Exit(1)
					}
				}
				if *check {
					fmt.Fprintf(os.Stderr, "ptyhelp patch: %s is stale (marker %q)\n", tp, *marker)
					os.Exit(1)
				}
			}
		} else {
			if err := mdpatch.PatchMarkdownFile(tp, stdout, *marker, patchOpts); err != nil {
				fmt.Fprintf(os.Stderr, "ptyhelp patch: %v\n", err)
				os.Exit(1)
			}
			writeOptionalStdoutFile("ptyhelp patch", stdout, *outPath)
		}
	}

	writeChildStderr("ptyhelp patch", stderr)

	if exitCode != 0 {
		os.Exit(exitCode)
	}
}
