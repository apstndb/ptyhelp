# ptyhelp

Two **subcommands** with separate roles:

| Subcommand | Purpose |
|------------|---------|
| **`ptyhelp run`** | Run a command in a **fixed-size pseudo-terminal** so CLIs that wrap to the terminal width (e.g. [go-flags](https://github.com/jessevdk/go-flags); [issue #423](https://github.com/jessevdk/go-flags/issues/423)) produce stable output. On Unix, stdout and stderr stay separate. |
| **`ptyhelp patch`** | Run a command and replace a `<!-- NAME begin -->` … `<!-- NAME end -->` region in a Markdown file with the command’s **stdout**. **`PTY` capture** is used if you pass **`-pty`**, **`-cols`**, or **`-rows`** (the latter two imply a PTY without `-pty`). Otherwise the child uses **plain pipes** and the **inherited environment**. |

There is **no default command** — you must pass the subprocess explicitly (e.g. `go run . --help`).

## Install

```bash
go install github.com/apstndb/ptyhelp@latest
```

## Command reference (generated)

The blocks below are produced with **`ptyhelp patch`** so this README stays aligned with the binary. From the repository root:

```bash
make readme
```

### Top-level (`ptyhelp help`)

<!-- readme-help begin -->
```text
ptyhelp — run a command in a PTY, or patch a Markdown marker from command stdout.

usage:
  ptyhelp run     [flags] command args...
  ptyhelp patch   [flags] command args...
  ptyhelp version
  ptyhelp help
```
<!-- readme-help end -->

### `ptyhelp run`

<!-- readme-run-help begin -->
```text
usage: ptyhelp run [flags] command args...

  -cols uint
    	PTY width in columns (default 256)
  -combined
    	merge stderr into stdout (same as -stderr=merge)
  -kill-after string
    	grace period after timeout before SIGKILL (e.g. 5s)
  -max-output-bytes int
    	fail when stdout or stderr exceeds this many bytes (0 = unlimited)
  -normalize-eol string
    	normalize line endings in captured stdout: none, lf, crlf (default "none")
  -o string
    	write child stdout to this file instead of printing it
  -rows uint
    	PTY height in rows (default 40)
  -stderr string
    	stderr handling: separate, merge, or discard (default "separate")
  -timeout string
    	maximum subprocess runtime (e.g. 30s, 5m)

Runs the command in a pseudo-terminal with the given size (Unix: stdout and stderr stay separate).
```
<!-- readme-run-help end -->

### `ptyhelp patch`

<!-- readme-patch-help begin -->
```text
usage: ptyhelp patch [flags] command args...

  -check
    	exit 1 when the target file would change (CI staleness check)
  -cols uint
    	PTY width (setting this flag implies PTY capture; cannot combine with -pty=false) (default 256)
  -combined
    	merge stderr into stdout (same as -stderr=merge)
  -dry-run
    	print the patched file to stdout when it would change, without writing
  -fence string
    	fenced code block language: text, none, or a language tag (default "text")
  -file string
    	markdown file to patch (required)
  -kill-after string
    	grace period after timeout before SIGKILL (e.g. 5s)
  -marker string
    	HTML comment name between <!-- NAME begin --> and <!-- NAME end --> (default "cli-output")
  -max-output-bytes int
    	fail when stdout or stderr exceeds this many bytes (0 = unlimited)
  -normalize-eol string
    	normalize line endings in the entire target file: none, lf, crlf (default "none")
  -o string
    	also write child stdout to this file (skipped when the child exits non-zero)
  -pty
    	run in a pseudo-terminal (redundant if -cols or -rows is set)
  -rows uint
    	PTY height (setting this flag implies PTY capture; cannot combine with -pty=false) (default 40)
  -stderr string
    	stderr handling: separate, merge, or discard (default "separate")
  -timeout string
    	maximum subprocess runtime (e.g. 30s, 5m)

Replaces the lines between <!-- MARKER begin --> and <!-- MARKER end --> with captured output.
Use -fence=none for raw Markdown, or command "-" to read patch content from stdin.
Child stderr is copied to stderr when separated (e.g. on Unix or non-PTY mode).
The target file is not modified when the child exits non-zero.
Note: in PTY mode on non-Unix platforms, stderr is typically merged into stdout.
```
<!-- readme-patch-help end -->

## Examples

### `ptyhelp run`

```bash
cd /path/to/your/module
ptyhelp run -- go run . --help
```

### `ptyhelp patch`

Requires **`-file`**. PTY-backed capture when **`-pty`** is set, or when **`-cols`** or **`-rows`** appears on the command line (those imply a PTY). **`-pty=false`** with **`-cols`/`-rows`** is an error.

Example (go-flags help; `-cols` implies PTY):

```bash
ptyhelp patch -file README.md -marker my-cli -cols 256 -- go run . --help
```

Pipe-only (no PTY):

```bash
ptyhelp patch -file README.md -marker my-cli -- go run . --help
```

## License

MIT — see [LICENSE](LICENSE).
