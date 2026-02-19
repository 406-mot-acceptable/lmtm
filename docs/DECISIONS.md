# Design Decisions

Architectural and design decisions for The Tunneler v2.
Each entry captures what was decided, why, and what alternatives were considered.

---

## 001 -- No config files, everything interactive

**Decision:** The Tunneler has no config files, no YAML, no bookmarks. Every session starts fresh with interactive prompts.

**Rationale:** The v1 tunneler was a config-file-driven bookmark manager that never became a reliable daily tool. Config files add complexity (file locations, format changes, migration) without adding value for a tool that connects to different sites with different devices each time.

**Alternatives Considered:**
- YAML config with saved gateway profiles -- rejected, adds maintenance burden and staleness
- SQLite bookmark store -- rejected, over-engineered for the use case

---

## 002 -- Password-only auth, prompted once in TUI

**Decision:** SSH authentication uses password only. No SSH key support. Password is prompted once at the start of the TUI session, held in memory, and zeroed on disconnect.

**Rationale:** Gateway devices (MikroTik, Ubiquiti) are typically accessed with password auth in the field. SSH keys add complexity (key management, agent forwarding) for a tool focused on quick interactive tunneling. Zeroing the password on disconnect prevents memory-resident credential leaks.

**Alternatives Considered:**
- SSH key auth -- rejected, adds complexity and is less common for gateway access
- SSH agent forwarding -- rejected, not needed for tunneling use case
- Keyring integration -- rejected, over-engineered

---

## 003 -- Port mapping formula: base + last octet

**Decision:** Local tunnel ports use a deterministic formula based on the device's last IP octet:
- Port 443 -> localhost:4430+octet (e.g., .5 -> 4435)
- Port 80  -> localhost:8030+octet (e.g., .5 -> 8035)
- Port 22  -> localhost:2230+octet (e.g., .5 -> 2235)
- Port 554 -> localhost:5540+octet (e.g., .5 -> 5545)

**Rationale:** Makes local ports memorable and predictable. "HTTPS on device .5 = 4435" is easy to remember. The base offsets (4430, 8030, 2230, 5540) are chosen to avoid common ports and provide 254 slots per service.

**Alternatives Considered:**
- Random high ports -- rejected, not memorable
- Sequential assignment -- rejected, changes between sessions
- User-specified ports -- rejected, conflicts with "no config" philosophy

---

## 004 -- Gateway auto-detection from SSH banner + command probes

**Decision:** Detect whether the gateway is MikroTik or Ubiquiti automatically after SSH connect. First check the SSH banner (MikroTik reports "ROSSSH"), then probe with `/system identity print`, fallback to Linux/Ubiquiti.

**Rationale:** Users shouldn't need to specify gateway type. The detection is cheap (one banner check + one command) and reliable. MikroTik has a distinctive banner; Ubiquiti runs standard Linux.

**Alternatives Considered:**
- User selects gateway type in TUI -- rejected, unnecessary friction
- SNMP detection -- rejected, requires additional protocol support

---

## 005 -- Device classification from MAC vendor OUI database

**Decision:** Classify discovered LAN devices by looking up their MAC address in an OUI vendor database. Known vendors map to device types:
- Hikvision/Dahua/Axis/Reolink -> Camera -> ports 22,80,443,554
- MikroTik/Ubiquiti/Cisco -> Router -> ports 22,80,443
- Unknown -> ports 80,443 (or custom)

**Rationale:** MAC vendor lookup is fast, requires no active probing of devices, and provides good-enough classification for port selection. Most IP cameras and routers have recognizable OUI prefixes.

**Alternatives Considered:**
- Active port scanning (nmap-style) -- rejected, too slow and intrusive
- mDNS/SSDP discovery -- rejected, not reliable on all networks
- Manual classification -- rejected, too much user friction

---

## 006 -- MikroTik: /ip arp print terse to avoid pagination

**Decision:** Use `/ip arp print terse` instead of `/ip arp print` when querying MikroTik ARP tables.

**Rationale:** The default MikroTik `print` output uses pagination that requires interactive terminal handling. The `terse` flag outputs machine-parseable one-line-per-entry format without pagination, which is much easier to parse programmatically.

**Alternatives Considered:**
- Standard print with pagination handling -- rejected, fragile terminal emulation
- API access -- rejected, requires separate API port and credentials

---

## 007 -- Ubiquiti: ssh-rsa host key algorithm fallback

**Decision:** When connecting to Ubiquiti devices, auto-retry with the `ssh-rsa` host key algorithm if the initial key exchange fails.

**Rationale:** Older Ubiquiti EdgeOS firmware uses RSA host keys that newer SSH clients reject by default (OpenSSH 8.8+ deprecated ssh-rsa). The fallback ensures compatibility without requiring users to modify their SSH config.

**Alternatives Considered:**
- Require users to configure SSH client -- rejected, violates "just works" principle
- Always use ssh-rsa -- rejected, weakens security for MikroTik and newer devices

---

## 008 -- Single SSH connection with multiplexed port forwards

**Decision:** All tunnels for a session share one SSH connection. Port forwards are multiplexed over the single connection.

**Rationale:** SSH supports multiplexing natively. One connection means one authentication, one keepalive, and clean teardown. Multiple connections would complicate credential management and increase load on the gateway.

**Alternatives Considered:**
- One connection per tunnel -- rejected, wasteful and harder to manage
- SSH ControlMaster -- rejected, relies on external OpenSSH, not portable

---

## 009 -- All tunnel listeners bind 127.0.0.1 only

**Decision:** Local tunnel endpoints bind exclusively to 127.0.0.1, never to 0.0.0.0 or other interfaces.

**Rationale:** Binding to all interfaces would expose tunneled services to the local network, creating a security risk. Localhost-only binding ensures only the local user can access the tunnels.

**Alternatives Considered:**
- Configurable bind address -- rejected, too easy to accidentally expose services
- Bind to 0.0.0.0 -- rejected, security risk

---

## 010 -- OSC8 hyperlinks for clickable URLs with graceful fallback

**Decision:** Active tunnel URLs are rendered using OSC8 escape sequences, making them Ctrl+clickable in supporting terminals. Falls back to plain text in terminals that don't support OSC8.

**Rationale:** Clicking `https://localhost:4435` to open a device's web interface is the fastest workflow. OSC8 is widely supported (iTerm2, Windows Terminal, GNOME Terminal, kitty, etc.) and degrades gracefully to visible plain text.

**Alternatives Considered:**
- Always plain text -- rejected, misses UX improvement opportunity
- Auto-open in browser -- rejected, too aggressive, user should choose when to open

---

## 011 -- ASCII pipe animation synchronized with real tunnel events

**Decision:** The tunnel construction animation uses ASCII pipe characters and is synchronized with actual tunnel establishment events. The pipe "completes" when the real port forward is listening.

**Rationale:** Purely cosmetic animations feel dishonest. Tying the visual to real events gives users accurate feedback about tunnel state while keeping the interface engaging.

**Alternatives Considered:**
- Simple spinner -- rejected, less informative and less engaging
- Progress bar -- rejected, tunnel establishment doesn't have clear percentage progress
- No animation -- rejected, tunnels build fast but the visual feedback is valuable

---

## 012 -- No Cobra CLI, no YAML -- just ./tunneler launches TUI

**Decision:** No CLI framework. The binary launches directly into the Bubbletea TUI. No subcommands, no flags, no arguments.

**Rationale:** The tool has exactly one mode of operation: interactive tunneling. A CLI framework adds dependency weight and API surface for zero benefit. `./tunneler` does one thing.

**Alternatives Considered:**
- Cobra with subcommands -- rejected, only one operation to perform
- Flag-based quick mode -- rejected, conflicts with interactive-only philosophy

---

## 013 -- Bubbletea for TUI, lipgloss for styling

**Decision:** Use the Charm stack (Bubbletea, Bubbles, lipgloss) for the terminal UI.

**Rationale:** Bubbletea's Elm architecture (Model-Update-View) maps well to the wizard flow. Lipgloss provides consistent cross-platform styling. The Charm ecosystem is the standard for Go TUIs with active maintenance and good documentation.

**Alternatives Considered:**
- tview -- rejected, imperative widget model is harder to reason about for wizard flows
- tcell directly -- rejected, too low-level
- Custom ANSI rendering -- rejected, reinventing the wheel

---

## 014 -- Go 1.22 with minimal dependencies

**Decision:** Use Go 1.22 (system constraint). Keep the dependency tree minimal:
- charmbracelet/bubbletea, bubbles, lipgloss (TUI)
- golang.org/x/crypto, x/term (SSH + terminal)
- endobit/oui (MAC vendor lookup)

**Rationale:** Fewer dependencies mean fewer supply chain risks, easier auditing, and faster builds. Every dependency must justify its presence. Originally targeted Go 1.24, but system has Go 1.22.2 installed. No Go 1.24-specific features are needed, so no functional impact.

**Alternatives Considered:**
- Embedded OUI database -- possible future optimization, but the library works for now
- Upgrading to Go 1.24 -- unnecessary, no features needed from newer versions

---

## 015 -- Interface contracts: CommandRunner matches ssh.Client.Exec

**Decision:** The `gateway.CommandRunner` interface is defined as `Exec(cmd string) (string, error)`, which matches the `ssh.Client.Exec` method signature exactly. Gateway implementations accept a `CommandRunner` to execute commands on the remote device.

**Rationale:** Clean dependency inversion. Gateway code depends on an interface, not the SSH client directly. This makes testing easier (mock the interface) and keeps the dependency graph one-directional: gateway -> interface <- ssh.

**Alternatives Considered:**
- Gateway imports ssh package directly -- rejected, creates circular dependency risk
- Generic command executor with context -- rejected, over-engineered for the use case

---

## 016 -- TunnelEvent channel for real-time TUI updates

**Decision:** `ssh.Manager` emits `TunnelEvent` structs on a Go channel as tunnels are established, failed, or closed. The TUI subscribes to this channel to drive the ASCII pipe animation in real time.

**Rationale:** Decouples the tunnel engine from the UI. The manager doesn't know about Bubbletea; it just sends events. The TUI converts events to Bubbletea messages. This is the standard Bubbletea pattern for external event sources (tea.Cmd that reads from a channel).

**Alternatives Considered:**
- Callback functions -- rejected, harder to integrate with Bubbletea's message-based model
- Polling tunnel status -- rejected, introduces latency and unnecessary CPU usage

---

## 017 -- Gateway structs in dedicated files

**Decision:** In Phase 2, MikroTik and Ubiquiti gateway struct definitions and their method implementations are placed in dedicated files: `gateway/mikrotik.go` and `gateway/ubiquiti.go`. The shared interface stays in `gateway/gateway.go`.

**Rationale:** Keeps each gateway implementation self-contained and easy to navigate. Adding a new gateway type means adding one new file without touching existing implementations. The detect.go file handles the factory/selection logic.

**Alternatives Considered:**
- All gateway code in one file -- rejected, would grow too large and mix concerns
- Separate packages per gateway -- rejected, over-engineered for two implementations

---

## 018 -- endobit/oui uses compiled-in IEEE database

**Decision:** The `endobit/oui` library embeds the IEEE OUI database at compile time. No runtime file loading, no external data files, no download step.

**Rationale:** Simplifies deployment -- the binary is fully self-contained. No need to ship or locate a separate OUI data file. The compiled-in database is sufficient for device classification (camera/router/unknown) since we only need to match a handful of well-known vendors, not resolve every MAC address.

**Alternatives Considered:**
- Runtime OUI file loading -- rejected, adds file path complexity and failure modes
- Downloading OUI database on first run -- rejected, requires network access and adds latency
- Hardcoded vendor prefixes -- rejected, less maintainable than using the library

---

## 019 -- TUI uses local WANConfig/LANConfig copies

**Decision:** The TUI package defines its own `WANConfig` and `LANConfig` structs that mirror the gateway package types, rather than importing gateway types directly.

**Rationale:** Avoids circular import issues. The TUI package should not depend on the gateway package -- it receives data through the app orchestration layer. Local type copies keep the dependency graph clean: gateway -> app -> tui. The structs are simple data containers with the same fields, so duplication is minimal.

**Alternatives Considered:**
- TUI imports gateway package directly -- rejected, creates tight coupling and risks circular imports
- Shared types package -- rejected, premature abstraction for two small structs
- Interface-based approach -- rejected, over-engineered for data transfer objects

---

## 020 -- ValidateSubnet at gateway command boundaries (defense in depth)

**Decision:** Add `ValidateSubnet()` validation at the `FloodPing` and `ARPTable` method boundaries in both MikroTik and Ubiquiti gateway implementations. The subnet parameter is validated with a CIDR regex before being interpolated into any shell command string.

**Rationale:** Defense in depth. The subnet parameter typically comes from the gateway's own `LANInfo()` output, so it should always be a valid CIDR. However, the gateway command methods are public API boundaries -- they accept a string that gets embedded in shell commands (`ping` on Ubiquiti, `/ping` on MikroTik). Validating at the boundary prevents command injection even if the caller passes untrusted input. Found during the Phase 3 security review.

**Alternatives Considered:**
- Trust the caller (subnet always comes from gateway) -- rejected, violates defense in depth principle
- Parameterized command execution -- not available on MikroTik RouterOS, which uses a proprietary CLI
- Separate subnet type with parse-time validation -- rejected, over-engineered for a single validation point

---

## 021 -- TUI defines own wizardState to break import cycle

**Decision:** The `tui` package defines its own `wizardState` type (unexported) that mirrors the `app.WizardState` enum, rather than importing from the `app` package. The `tui/app.go` file orchestrates state transitions internally using this private enum. The `app` package's `WizardState` is for external use by callers outside the TUI; the TUI's `wizardState` is internal only.

**Rationale:** Direct import of `app` from `tui` creates a circular dependency: `app` imports `tui` (to create the root model), and `tui` would import `app` (for state enum). Go forbids import cycles. The TUI's local enum breaks the cycle while keeping the state machine concept consistent across both packages. The mapping is a simple switch statement in one place.

**Alternatives Considered:**
- Shared types package for WizardState -- rejected, premature abstraction for a single enum
- TUI imports app via interface -- rejected, doesn't solve the cycle since the interface would still live in app
- Merge app and tui into one package -- rejected, conflates orchestration with presentation

---

## 022 -- Manager captured as pointer in tea.Cmd closures

**Decision:** When creating `tea.Cmd` functions that read from `ssh.Manager` (e.g., listening for TunnelEvents), the manager is created in `updateDevices` (not inside the `tea.Cmd` closure itself) and captured as a pointer (`*ssh.Manager`) in the `buildCmd`/`nextEventCmd` closures before the closure runs. This is critical because Bubbletea models use value receivers -- if the manager were created inside the closure, it would be a fresh value on each call.

**Rationale:** Go closures capture variables, not values. If the manager struct were copied into the closure (e.g., by passing it as a value parameter to a helper), the closure would read stale state -- it wouldn't see tunnels established after the closure was created. Capturing the pointer ensures the TUI always reads the current tunnel state. This bug was found during Phase 4 wiring when the tunnel dashboard showed zero tunnels despite successful establishment.

**Alternatives Considered:**
- Pass manager by value and re-query -- rejected, defeats the purpose of the event channel
- Use a channel-only approach with no direct manager access -- rejected, the dashboard also needs to read current tunnel list for rendering

---

## 023 -- ValidateSubnet for command injection prevention

**Decision:** All gateway methods that interpolate subnet strings into shell commands (`FloodPing`, `ARPTable`) validate the input matches a strict CIDR regex (`^\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}/\d{1,2}$`) before executing any command. Invalid inputs return an error immediately.

**Rationale:** Defense in depth. The subnet parameter normally comes from the gateway's own `LANInfo()` output and should always be valid CIDR. However, `FloodPing` and `ARPTable` are public API boundaries where the string gets embedded directly into shell commands (`ping` on Ubiquiti, `/ping` on MikroTik). Validating at this boundary prevents command injection even if a future caller passes untrusted input. Identified during the Phase 3 security review.

**Alternatives Considered:**
- Trust the caller (subnet always comes from gateway) -- rejected, violates defense in depth
- Parameterized command execution -- not available on MikroTik RouterOS proprietary CLI
- Separate subnet type with parse-time validation -- rejected, over-engineered for a single validation point
