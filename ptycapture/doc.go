// Package ptycapture runs subprocesses with optional pseudo-terminal sizing,
// timeouts, output limits, and stderr handling for documentation tools.
//
// Plain capture bounds output draining after the direct child exits so that a
// descendant retaining an inherited pipe cannot block forever. Output written
// through such descendant-held pipes may therefore be truncated; the drain
// interval is an implementation detail, not a timing guarantee.
package ptycapture
