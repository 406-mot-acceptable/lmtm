# The Tunneler v2 -- Implementation Plan

## Context

The previous tunneler was a config-file-driven bookmark manager that never became a reliable daily tool. The new version is an **interactive tunnel builder** -- no config files, no bookmarks. You SSH into a gateway, it figures out what's there, you pick devices, tunnels get built. Done.

## User Flow

```
Enter IP + Password
       |
   SSH Connect
       |
  Auto-detect MikroTik or Ubiquiti
       |
  Query WAN/LAN settings -> display
       |
  Flood ping subnet + read ARP
       |
  List devices (MAC vendor, classification)
       |
  Select devices (first 10 / pick specific)
       |
  Classify ports (cam=22,80,443,554 / router=22,80,443 / custom)
       |
  Build tunnels (ASCII pipe animation)
       |
  Active tunnel view (clickable https:// hyperlinks)
```

## Project Structure

```
the_tunneler/
  cmd/tunneler/main.go            -- entry point, launches TUI
  internal/
    app/
      state.go                    -- WizardState enum + transitions
      app.go                      -- orchestration, wires subsystems
    ssh/
      client.go                   -- SSH connection, keepalive, password auth
      exec.go                     -- run commands on gateway
      tunnel.go                   -- single port forward lifecycle
      manager.go                  -- manages all tunnels on one connection
    gateway/
      gateway.go                  -- Gateway interface (WAN, LAN, ARP, ping)
      detect.go                   -- auto-detect MikroTik vs Ubiquiti
      mikrotik.go                 -- RouterOS commands + parsing
      ubiquiti.go                 -- EdgeOS/Linux commands + parsing
    discovery/
      scanner.go                  -- orchestrates ping + ARP + classify
      arp.go                      -- ARP table parsing (both formats)
      classify.go                 -- device type from MAC vendor
      vendor.go                   -- MAC OUI lookup wrapper
      device.go                   -- DiscoveredDevice struct, DeviceClass enum
    portmap/
      portmap.go                  -- port mapping formula + collision handling
    tui/
      app.go                      -- root Bubbletea model, state machine
      connect.go                  -- IP + password input screen
      detect.go                   -- detection progress screen
      survey.go                   -- WAN/LAN info display
      scan.go                     -- scan progress with spinner
      devices.go                  -- device list with selection
      building.go                 -- tunnel construction view
      tunnels.go                  -- active tunnel dashboard
      animation.go                -- ASCII pipe construction frames
      styles.go                   -- lipgloss color palette
      keys.go                     -- keybindings
      components/
        spinner.go                -- reusable spinner
        table.go                  -- styled table
        hyperlink.go              -- OSC8 clickable URLs
  go.mod
  Makefile
```

## Key Design Decisions

**No config file.** Everything is interactive. No Cobra CLI -- just `./tunneler`.

**Port mapping formula:**
- Port 443 -> localhost:4430+octet (e.g., .5 -> 4435)
- Port 80  -> localhost:8030+octet (e.g., .5 -> 8035)
- Port 22  -> localhost:2230+octet (e.g., .5 -> 2235)
- Port 554 -> localhost:5540+octet (e.g., .5 -> 5545)

Memorable: "HTTPS on device .5 = 4435"

**Device classification from MAC vendor:**
- Hikvision/Dahua/Axis/Reolink -> Camera -> ports 22,80,443,554
- MikroTik/Ubiquiti/Cisco -> Router -> ports 22,80,443
- Unknown -> ports 80,443 (or custom)

**Gateway detection:** SSH banner check first (MikroTik reports "ROSSSH"), then command probe (`/system identity print`), fallback to Linux/Ubiquiti.

**Password handling:** Prompted once at start, held in memory, zeroed on disconnect. No SSH keys.

**Ubiquiti quirk:** Auto-retry with `ssh-rsa` host key algorithm if key exchange fails.

**MikroTik ARP:** Use `/ip arp print terse` to avoid pagination. Skip flood-ping if ARP table is already populated.

**ASCII tunnel animation:** Synchronized with real tunnel events -- pipe "completes" when actual port forward is listening.

**OSC8 hyperlinks:** `https://localhost:4432` rendered as Ctrl+clickable in supporting terminals. Falls back to plain text gracefully.

## Agent Team (6 agents, hybrid workflow)

1. **TUI Design** -- Bubbletea views, animation, styling, UX flow
2. **Security** -- SSH client, tunnel engine, password handling, secure defaults
3. **Compatibility** -- MikroTik + Ubiquiti command abstraction, edge cases
4. **Backend** -- state machine, discovery, port mapping, wiring, orchestration
5. **Devil's Advocate** -- reviews all code, finds flaws, stress-tests assumptions
6. **Note Taker** -- captures decisions, progress, and maintains the Kanban board

## Build Phases

**Phase 1 -- Foundation (parallel, no deps):**
- Security agent: `ssh/client.go`, `ssh/exec.go`
- Compatibility agent: `gateway/gateway.go`, `gateway/detect.go`
- Backend agent: `app/state.go`, `portmap/portmap.go`, `go.mod`, `Makefile`
- TUI agent: `tui/styles.go`, `tui/keys.go`, `tui/components/`
- Note Taker: sets up docs, records initial design decisions

**Phase 2 -- Core (parallel, depends on Phase 1):**
- Security: `ssh/tunnel.go`, `ssh/manager.go`
- Compatibility: `gateway/mikrotik.go`, `gateway/ubiquiti.go`
- Backend: `discovery/` package (device, classify, vendor, arp)
- TUI: `tui/connect.go`, `tui/survey.go`, `tui/scan.go`

**Phase 3 -- Integration (parallel, depends on Phase 2):**
- Backend: `discovery/scanner.go` (uses gateway + ssh)
- TUI: `tui/devices.go`, `tui/building.go`, `tui/animation.go`, `tui/tunnels.go`
- Security: review of all gateway + discovery code

**Phase 4 -- Wiring (sequential):**
- Backend + TUI: `tui/app.go`, `app/app.go`, `cmd/tunneler/main.go`

**Phase 5 -- Polish:**
- Devil's Advocate: full review pass
- All: integration testing, animation polish

## Demo Success Criteria

The demo is "working" when:
1. `./tunneler` launches the TUI
2. Enter MikroTik gateway IP + password -> connects
3. Auto-detects MikroTik, shows identity
4. Displays WAN IP, LAN subnet, DHCP range
5. Scans network, lists devices with vendor names
6. Select 3 devices -> classification assigns ports
7. ASCII pipe animation plays during tunnel build
8. Active view shows tunnels with clickable URLs
9. Ctrl+click opens browser to device web interface
10. `q` disconnects cleanly, exits

Also works with Ubiquiti EdgeRouter (ssh-rsa fallback).

## Dependencies

```
github.com/charmbracelet/bubbletea
github.com/charmbracelet/bubbles
github.com/charmbracelet/lipgloss
golang.org/x/crypto
golang.org/x/term
github.com/endobit/oui
```

No Cobra. No YAML. No config file.
