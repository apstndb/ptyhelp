# Agent instructions for `ptyhelp`

## Build, test, and lint commands

- Full test suite: `go test ./...`
- Make target for the full test suite: `make test`
- Run a single test from the root package: `go test -run '^TestRunSubcommandPropagatesExitCode$' .`
- Run a specific Markdown patching test: `go test -run '^TestPatchMarkdownFileRequiresExactMarkerLines$' ./mdpatch`
- Build the CLI binary from the repository root: `go build -o /tmp/ptyhelp .`
- Lint command used in CI: `golangci-lint run`
- Refresh the generated CLI help blocks in `README.md`: `make readme`

## High-level architecture

- `main.go` is the CLI entrypoint. It exposes two subcommands with different pipelines:
  - `ptyhelp run` always captures a subprocess through the PTY path and optionally normalizes line endings before writing stdout or a file.
  - `ptyhelp patch` chooses between PTY capture, plain pipe capture, or stdin (`-` command); then patches a Markdown marker block with captured output.
- Public importable packages:
  - `mdpatch` — marker-block replacement, `-fence` modes, EOL normalization, and `BuildPatchedContent` for `--check` / `--dry-run`.
  - `ptycapture` — `CapturePTY`, `CapturePlain`, stderr merge/discard, timeout, kill-after, and max-output-bytes limits.
- Output capture is split by platform inside `ptycapture`:
  - `ptycapture/plain.go` is the non-PTY code path and preserves ordinary stdin/stdout/stderr pipe behavior.
  - `ptycapture/pty_unix.go` is the Unix PTY implementation using `creack/pty`. It keeps stdout on a PTY for stable wrapping and captures stderr separately.
  - `ptycapture/pty_windows.go` uses `charmbracelet/x/conpty` (see `docs/WINDOWS_PTY.md`). stderr is merged into stdout on Windows PTY.
  - `ptycapture/pty_other.go` returns an unsupported error on other platforms.
- Platform-specific helpers are intentionally split with build tags where needed.
- `main.go` delegates capture to `ptycapture` and patching to `mdpatch`.

## Key conventions

- `README.md` contains generated help blocks between HTML markers. If a flag, usage string, or help output changes, regenerate those sections with `make readme` instead of editing the generated blocks by hand.
- `mdpatch.PatchMarkdownFile` only recognizes marker lines when `strings.TrimSpace(line)` exactly matches `<!-- NAME begin -->` / `<!-- NAME end -->`. Text that merely contains a marker substring is not a match, and duplicate begin markers are treated as errors.
- `mdpatch.NormalizeEOL` and `PatchMarkdownFile` preserve standalone `\r` characters. In `EOLNone` mode, CRLF is preserved only when the target file is consistently CRLF; mixed-EOL files are rewritten with LF.
- `ptyhelp patch` must not rewrite the target file when the child command exits non-zero. Preserve that behavior when changing patch flow or exit handling.
- `-fence=none` inserts raw content between markers without a fenced code block. `-fence=<lang>` uses that language tag.
- `ptyhelp patch -` reads patch content from stdin instead of running a subprocess.
- `-stderr=merge` (or `-combined`) merges stderr into captured stdout for patch/run.
- `--check` exits 1 when the target file would change; `--dry-run` prints the would-be file to stdout without writing.
- PTY behavior on Unix: real terminals keep PTY stdin semantics; redirected stdin should pass through unchanged when implemented.
- Integration-style tests in `main_test.go` and `capture_unix_test.go` use shared helpers like `runBuiltCommand` and `buildTestBinary` to keep subprocess tests deterministic and bounded by timeouts. Reuse those helpers for new subprocess tests instead of calling `exec.Command(...).CombinedOutput()` directly.
