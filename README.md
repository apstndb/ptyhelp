# ptyhelp

Run a command inside a [pseudo-terminal](https://github.com/aymanbagabas/go-pty) with a **fixed width** so tools that wrap help text to the terminal size (for example [go-flags](https://github.com/jessevdk/go-flags); see [issue #423](https://github.com/jessevdk/go-flags/issues/423)) produce **stable, embeddable output** for READMEs and docs.

Typical use: capture `go run . --help`, or any `cmd args`, optionally writing stdout to a file and/or replacing a fenced block in a Markdown file between HTML comment markers.

## Install

```bash
go install github.com/apstndb/ptyhelp@v0.1.0
```

## Usage

```text
ptyhelp [flags] [-- command args...]
```

If you omit `command args`, the default is `go run . --help`. If you set `-binary BIN`, the default is `BIN --help`.

Flags:

| Flag | Meaning |
|------|---------|
| `-cols`, `-rows` | PTY size (default 256×40) |
| `-o path` | Write raw output to a file |
| `-target-file path` | Replace the `<!-- NAME begin -->` … `<!-- NAME end -->` region in that file with a fenced `text` code block (see `-marker`) |
| `-marker NAME` | Marker name (default `cli-output`) |

Example:

```bash
cd /path/to/your/module
ptyhelp -o /tmp/help.txt -target-file README.md -marker my-cli -- go run . --help
```

## Release

1. Create the GitHub repository `github.com/apstndb/ptyhelp` (empty, no README commit, or merge histories if needed).
2. **Before pushing**, add `origin` and compare: `git remote add origin git@github.com:apstndb/ptyhelp.git` (once), then `git fetch origin` and `git diff origin/main` (after the first push, `origin/main` exists; on the very first push there is no remote branch yet—review with `git log` / `git show` instead).
3. `git push -u origin main` and `git push origin v0.1.0` so `go run github.com/apstndb/ptyhelp@v0.1.0` resolves from the module proxy.

## License

MIT — see [LICENSE](LICENSE).
