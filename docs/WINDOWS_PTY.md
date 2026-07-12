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

On timeout, Windows ConPTY capture waits for the configured `-kill-after`
grace period before calling `TerminateProcess`; Windows does not provide the
Unix `SIGTERM` to `SIGKILL` escalation used by the Unix implementation. During
shutdown, output remains actively drained while the pseudoconsole closes. This
ordering is required to avoid `ClosePseudoConsole` deadlocks on Windows versions
before Windows 11 24H2.

When input reaches EOF, `ptyhelp` sends the Windows console EOF sequence
(`Ctrl+Z` followed by Enter) through ConPTY. It does not close the ConPTY input
handle while the child is running, because that generates a control-close event
and terminates the child.

Unix builds continue to use `github.com/creack/pty`.
