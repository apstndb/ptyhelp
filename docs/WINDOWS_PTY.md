# Windows PTY dependency

## Decision

`ptyhelp` replaced `github.com/aymanbagabas/go-pty` with
[`github.com/charmbracelet/x/conpty`](https://pkg.go.dev/github.com/charmbracelet/x/conpty)
for Windows PTY capture (`ptycapture/pty_windows.go`).

## Rationale

| Dependency | Direct runtime deps | Notes |
|------------|--------------------|-------|
| `go-pty` | `golang.org/x/crypto/ssh`, `golang.org/x/sys`, `u-root/u-root` | Pulls SSH stack into Windows builds |
| `charmbracelet/x/conpty` | `golang.org/x/sys` only | Native ConPTY wrapper, maintained by Charm |

Windows PTY remains a supported feature: `ptyhelp run` and `ptyhelp patch -pty`
work on Windows via ConPTY. On non-Unix, non-Windows platforms PTY capture
returns a clear unsupported error.

Unix builds continue to use `github.com/creack/pty`.
