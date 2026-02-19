---

kanban-plugin: basic

---

## Backlog

- [ ] Manual device entry -- custom LAN IP + port when scan finds nothing @tui @backend
- [ ] Push to GitHub (init repo, .gitignore, commit, create remote)

## In Progress

## Review

## Done

- [x] Set up Go module and project scaffold @backend
- [x] SSH client with password auth and keepalive @security
- [x] Remote command execution wrapper @security
- [x] Gateway interface definition @compatibility
- [x] Gateway auto-detection (MikroTik vs Ubiquiti) @compatibility
- [x] WizardState enum and transition logic @backend
- [x] Port mapping formula and collision handling @backend
- [x] TUI styles and color palette @tui
- [x] TUI keybindings @tui
- [x] Reusable components (spinner, table, hyperlink) @tui
- [x] Set up docs/DECISIONS.md @notes
- [x] Set up docs/PROGRESS.md @notes
- [x] SSH tunnel port forward lifecycle @security
- [x] Tunnel manager (multi-tunnel on one connection) @security
- [x] MikroTik command implementation (WAN, LAN, ARP, ping) @compatibility
- [x] Ubiquiti command implementation (WAN, LAN, ARP, ping) @compatibility
- [x] Device struct and DeviceClass enum @backend
- [x] MAC vendor lookup wrapper @backend
- [x] Device classification from vendor @backend
- [x] ARP table parsing (both formats) @backend
- [x] Connect screen (IP + password input) @tui
- [x] Detection progress screen @tui
- [x] Survey screen (WAN/LAN display) @tui
- [x] Scan progress screen @tui
- [x] Discovery scanner orchestration @backend
- [x] Device selection screen @tui
- [x] Tunnel building screen with ASCII animation @tui
- [x] Active tunnel dashboard with clickable URLs @tui
- [x] Security review pass @security @devils-advocate
- [x] Root TUI model and state machine @backend
- [x] Application orchestration and wiring @backend
- [x] Main entry point @backend
- [x] Integration testing @all
- [x] Final docs @notes
- [x] Code review and cleanup (6 fixes) @devils-advocate
- [x] TUI visual overhaul -- LMTM banner, panels, tunnel diagrams @tui
- [x] Fix Ubiquiti airOS 8 LAN subnet display bug @compatibility
- [x] Violet color palette + monotone frame overhaul @tui
- [x] Rotating taglines on LMTM banner @tui
- [x] Tunnel counter easter egg (milestones at 100/500/1000) @backend
- [x] Quick polish -- preset hint, build summary, spinner colors @tui
- [x] Fix Ubiquiti airOS 8 LAN/WAN detection (system.cfg + ifconfig + arp) @compatibility
- [x] Tested on MikroTik + Ubiquiti airOS 8 gateways @all

## Blocked
