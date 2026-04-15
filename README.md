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

## Usage

```text
ptyhelp run   [flags] command args...
ptyhelp patch [flags] command args...
ptyhelp help
```

### `ptyhelp run`

Runs the child **always** in a PTY (`-cols` / `-rows`, default 256×40).

| Flag | Meaning |
|------|---------|
| `-cols`, `-rows` | PTY size |
| `-o path` | Write child stdout to a file instead of printing it |
| `-normalize-eol MODE` | Normalize line endings: `none` (default), `lf`, `crlf` |

Example:

```bash
cd /path/to/your/module
ptyhelp run -- go run . --help
```

### `ptyhelp patch`

Requires **`-file`**. PTY-backed capture when **`-pty`** is set, or when **`-cols`** or **`-rows`** appears on the command line (those imply a PTY). **`-pty=false`** with **`-cols`/`-rows`** is an error.

| Flag | Meaning |
|------|---------|
| `-file path` | Markdown file to patch (**required**) |
| `-marker NAME` | Marker name (default `cli-output`) |
| `-pty` | Run the child in a PTY (redundant if `-cols` or `-rows` is set) |
| `-cols`, `-rows` | PTY size; **setting either implies PTY** (default 256×40 if omitted) |
| `-o path` | Also write child stdout to this file |
| `-normalize-eol MODE` | Line-ending normalization (CRLF vs LF). **none** (default) preserves raw output for **-o**, but for **patch** it matches the target file's perceived style (if consistent) to avoid mixed line endings; defaults to **LF** for mixed-EOL files. **lf** and **crlf** apply to captured stdout **and**, for **patch**, rewrite the **entire Markdown file on disk** to that EOL style (not only the fenced block). *Note: Standalone carriage returns (e.g. progress bars) are preserved internally, though leading/trailing whitespace and line endings from the captured output are trimmed before insertion. On non-Unix platforms, PTY capture may merge stderr into stdout.* |

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
