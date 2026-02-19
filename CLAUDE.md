# CLAUDE.md

## Project

LMTM (Lean Mean Tunneling Machine) -- Interactive SSH tunnel builder in Go with a Bubbletea TUI.
Connects to MikroTik or Ubiquiti gateways, discovers LAN devices, builds SSH port forwards.

## Architecture

See docs/ARCHITECTURE.md for full details.

- Go 1.22+, Bubbletea TUI, no config files, no Cobra CLI
- Password auth only (no SSH keys)
- Auto-detect gateway type from SSH banner + command probes
- Port mapping: 4430+octet (443), 8030+octet (80), 2230+octet (22), 5540+octet (554)
- ASCII pipe animation during tunnel construction
- OSC8 clickable hyperlinks in active tunnel view
- Violet accent palette on monotone dark frame

## Project Layout

```
cmd/tunneler/main.go        -- entry point
internal/app/                -- state machine, orchestration
internal/ssh/                -- SSH client, tunnels, command exec
internal/gateway/            -- MikroTik + Ubiquiti abstraction
internal/discovery/          -- network scanning, ARP, classification
internal/portmap/            -- port mapping formula
internal/stats/              -- persistent tunnel counter
internal/tui/                -- all Bubbletea views and components
docs/                        -- architecture, decisions, progress, review
```

## Style

- No emojis
- No config files -- everything is interactive
- Keep code simple, no over-engineering
- Violet (#AF87FF) primary accent, monotone gray borders (#3A3A3A)
- Semantic colors only: green (success), red (error), yellow (warning)

## Key Constraints

- Password auth only, prompted in TUI at start
- Ubiquiti airOS 8: parse /tmp/system.cfg for LAN detection, ifconfig/arp fallback
- Ubiquiti EdgeOS: ssh-rsa host key algorithm fallback
- MikroTik: use `/ip arp print terse` to avoid pagination
- All tunnel listeners bind to 127.0.0.1 only
- Zero password after disconnect
- No InsecureIgnoreHostKey -- display fingerprint on first connect
- Module path: github.com/406-mot-acceptable/lmtm
