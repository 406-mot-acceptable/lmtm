# Code Review -- Non-TUI Packages

Review of `internal/ssh/`, `internal/gateway/`, `internal/discovery/`,
`internal/app/`, `internal/portmap/`, and `cmd/tunneler/main.go`.

---

## Summary

The codebase is well-structured. No critical security issues found. The
following issues were identified and fixed or noted.

---

## Fixes Applied

### 1. ssh/client.go -- Remove dead `bytesEqual` function

**Severity:** IMPORTANT (dead code)

The `bytesEqual` function (was at line 226) was never used anywhere in the
codebase. The actual host key comparison uses `subtle.ConstantTimeCompare`
(line 103), which is the correct constant-time approach. Removed the dead
function.

### 2. ssh/client.go -- Guard `StartKeepalive` against nil context

**Severity:** IMPORTANT (panic prevention)

If `StartKeepalive` were called before `Connect` (e.g., due to a future code
change), `c.ctx` would be nil, causing a panic on `<-c.ctx.Done()`. Added a
nil-context guard that returns early if `ctx` is not set. Currently the only
caller (`tui/app.go:406`) calls it after `Connect`, so this is defensive.

### 3. ssh/tunnel.go -- Add backoff to `acceptLoop` on persistent errors

**Severity:** IMPORTANT (goroutine spin prevention)

The accept loop would spin in a tight loop on persistent errors (e.g., file
descriptor exhaustion) because the `default` branch immediately continued.
Added:
- Incremental backoff: `consecutiveErrors * 50ms` sleep per error.
- Hard limit: after 10 consecutive errors, mark tunnel as failed and exit.
- Counter resets on each successful accept.

### 4. gateway/gateway.go -- Check `fmt.Sscanf` return value in `ValidateSubnet`

**Severity:** IMPORTANT (input validation correctness)

`fmt.Sscanf` return value was ignored. If the format somehow didn't match
(despite the regex pre-check), the octet variables would default to 0, which
passes the `> 255` check. Now checks that exactly 3 items were parsed:
`n != 3` triggers an error.

### 5. gateway/mikrotik.go -- Hoist fallback regexes to package level

**Severity:** MINOR (performance)

`parseTerseARPFallback` compiled two regexes (`ipRe`, `macRe`) on every
invocation. Hoisted to package-level `var` declarations (`fallbackIPRe`,
`fallbackMACRe`) so they compile once at init.

### 6. gateway/ubiquiti.go -- Hoist fallback regexes to package level

**Severity:** MINOR (performance)

Same issue as mikrotik.go. `parseNeighFallback` compiled regexes per call.
Hoisted to package-level `var` declarations (`neighFallbackIPRe`,
`neighFallbackMACRe`).

---

## Issues Noted (No Code Change)

### N1. ssh/client.go -- Password string lingers in Go heap

The `Connect` method takes `password string`. Even though `zeroPassword()`
wipes the `[]byte` copy, the original Go string passed to `Connect` and
to `gossh.Password()` cannot be zeroed (strings are immutable in Go). This
is inherent to Go's memory model. The current zeroing is best-effort and
correct for the constraint. No practical fix without changing the SSH
library's auth interface.

### N2. discovery/arp.go -- Duplicate ARP parsers

`ParseMikroTikARP` and `ParseLinuxARP` in `discovery/arp.go` are public
functions that duplicate parsing logic already in `gateway/mikrotik.go` and
`gateway/ubiquiti.go`. Neither is called from production code paths (the
gateway implementations parse internally via their `ARPTable` methods).

These parsers may be intended for testing or reuse. Kept as-is since they
are public API, but they are currently dead code. If unused long-term,
consider removing or consolidating with gateway parsers.

### N3. app/state.go -- `ValidTransition` is uncalled

The `ValidTransition` function is not called anywhere in the codebase. The
TUI manages its own state transitions in `tui/app.go`. However, the function
serves as documentation of the intended state machine and could be useful for
future testing. Kept as-is.

### N4. ssh/tunnel.go -- Tunnel fields not protected by mutex

`Tunnel.Status` and `Tunnel.Error` are read/written without synchronization.
Currently the TUI reads them from the main goroutine and they are only
written during `Start`/`Stop`/`acceptLoop`, which run sequentially relative
to reads. If the TUI ever polls tunnel status concurrently (e.g., ticker-
based refresh), this would become a data race. For now it is safe under
Bubbletea's single-threaded update model.

### N5. ssh/manager.go -- Silently dropped events

The `emit` method drops events if the channel buffer is full (non-blocking
send). This is documented and intentional -- the buffer is sized at
`len(specs)*2` which provides ample room. If a consumer is slow, events
could be lost. The TUI chains reads via `nextEventCmd`, so in practice the
channel drains promptly. No change needed, but worth noting for future
callers.

---

## What Looks Good

- **Password hygiene**: Zeroed on disconnect, never logged. Best-effort given
  Go string immutability.
- **Host key verification**: Proper TOFU with constant-time comparison and
  SHA256 fingerprint display. No `InsecureIgnoreHostKey`.
- **Command injection prevention**: `ValidateSubnet` called at all gateway
  command boundaries before string interpolation.
- **127.0.0.1 binding**: All tunnel listeners bind exclusively to loopback.
- **Goroutine lifecycle**: Keepalive goroutine exits on context cancel.
  Tunnel forwarding goroutines terminate via connection close propagation.
  Manager's `CloseAll` cancels the build context before stopping tunnels.
- **Channel hygiene**: Event channel is buffered, guarded against
  send-after-close with a separate mutex, and closed exactly once.
- **Error wrapping**: Consistent use of `fmt.Errorf` with `%w` throughout.
- **Port allocator**: Correct collision handling with 256-slot search space.
- **Clean dependency graph**: `gateway` depends on `CommandRunner` interface,
  not on `ssh` directly. No circular imports.

---

## Build Verification

```
$ go build ./...   # clean
$ go vet ./...     # clean
```
