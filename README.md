```
 ██╗     ███╗   ███╗████████╗███╗   ███╗
 ██║     ████╗ ████║╚══██╔══╝████╗ ████║
 ██║     ██╔████╔██║   ██║   ██╔████╔██║
 ██║     ██║╚██╔╝██║   ██║   ██║╚██╔╝██║
 ███████╗██║ ╚═╝ ██║   ██║   ██║ ╚═╝ ██║
 ╚══════╝╚═╝     ╚═╝   ╚═╝   ╚═╝     ╚═╝
 Lean Mean Tunneling Machine
```

Interactive SSH tunnel builder for MikroTik and Ubiquiti gateways. Connect to a gateway, discover LAN devices, pick what you want, and tunnels get built. No config files. No bookmarks. Just tunnels.

```
./lmtm
```

## What It Does

LMTM connects to a network gateway over SSH, figures out what devices are on the LAN, and builds local port forwards so you can reach them from your machine. The entire flow is driven by a terminal UI -- you never touch a config file.

```
Enter IP + Password
      |
  SSH Connect (auto-detects MikroTik or Ubiquiti)
      |
  Show WAN/LAN info
      |
  Scan network, list devices with vendor names
      |
  Select devices, assign port presets
      |
  Build tunnels (animated)
      |
  Active dashboard with clickable URLs
```

Port mapping is deterministic: device `.5` on port 443 always maps to `localhost:4435`. You learn the pattern once.

## Features

- Auto-detection of MikroTik RouterOS and Ubiquiti airOS/EdgeOS gateways
- MAC vendor lookup for device classification (cameras, routers, etc.)
- Deterministic port mapping formula (base + last octet)
- ASCII pipe animation synchronized with real tunnel events
- OSC8 clickable hyperlinks in supporting terminals
- Password auth with memory zeroing on disconnect
- Host key fingerprint verification (no InsecureIgnoreHostKey)
- All tunnel listeners bind to 127.0.0.1 only

## Installation

### Download a Binary

Grab a release from the [Releases](https://github.com/406-mot-acceptable/lmtm/releases) page.

### Build from Source

Requires Go 1.22 or later.

```bash
git clone https://github.com/406-mot-acceptable/lmtm.git
cd lmtm
make build
./lmtm
```

### Cross-Compile

```bash
make all    # builds linux/mac/windows for amd64 and arm64
```

## Usage

Run the binary. That's it. No flags, no arguments.

```bash
./lmtm
```

1. Enter the gateway IP, username, and password
2. LMTM connects and auto-detects the gateway type
3. Review the WAN/LAN survey, press Enter to scan
4. Select devices from the discovered list (Space to toggle, `a` for all, `f` for first 10)
5. Press `p` on a device to cycle port presets (Default/Camera/Router/Web)
6. Press Enter to build tunnels
7. Ctrl+click the URLs in the dashboard to open device web interfaces

### Port Mapping

| Remote Port | Local Port Formula | Example (.5)  |
|-------------|-------------------|---------------|
| 443 (HTTPS) | 4430 + octet      | localhost:4435 |
| 80 (HTTP)   | 8030 + octet      | localhost:8035 |
| 22 (SSH)    | 2230 + octet      | localhost:2235 |
| 554 (RTSP)  | 5540 + octet      | localhost:5545 |

### Keybindings

| Key | Action |
|-----|--------|
| Tab / Shift+Tab | Navigate input fields |
| Space | Toggle device selection |
| a / n | Select all / none |
| f | Select first 10 devices |
| p | Cycle port preset on selected device |
| Enter | Proceed to next step |
| Esc | Go back |
| q / Ctrl+C | Quit |

## Compatibility

### Operating Systems

| OS | Status |
|----|--------|
| Linux (amd64, arm64) | Supported |
| macOS (amd64, arm64) | Supported |
| Windows (amd64) | Supported (use Windows Terminal for best experience) |

### Gateways

| Device | Status | Notes |
|--------|--------|-------|
| MikroTik RouterOS | Tested | Uses `/ip arp print terse` for pagination-free output |
| Ubiquiti EdgeOS | Supported | Auto-retries with ssh-rsa for older firmware |
| Ubiquiti airOS 8 | Tested | Parses `/tmp/system.cfg`, falls back to ifconfig/arp |

### Terminals

OSC8 clickable hyperlinks work in: Windows Terminal, iTerm2, GNOME Terminal, kitty, Alacritty, WezTerm. Other terminals show plain text URLs.

## Project Structure

```
cmd/tunneler/          Entry point
internal/
  app/                 State machine, orchestration
  ssh/                 SSH client, tunnels, command execution
  gateway/             MikroTik + Ubiquiti abstraction
  discovery/           Network scanning, ARP, device classification
  portmap/             Port mapping formula
  stats/               Persistent tunnel counter
  tui/                 All Bubbletea views and components
    components/        Reusable spinner, table, hyperlink
docs/                  Architecture, decisions, progress log
```

## Design Philosophy

This project was largely vibe-coded with AI assistance (Claude). The design philosophy was:

- **Interactive over configurable.** No config files, no saved state, no bookmarks. Every session starts fresh.
- **One thing well.** The binary does exactly one thing: build SSH tunnels to LAN devices behind a gateway.
- **Honest UI.** The tunnel animation is synchronized with real events, not cosmetic filler.
- **Secure defaults.** Localhost-only binding, host key verification, password zeroing. No `InsecureIgnoreHostKey`.
- **Simple code.** Minimal dependencies, no frameworks beyond what's needed, no premature abstractions.

The codebase is a single-person project built in a few focused sessions. It works well for its intended use case but has not been through formal security auditing. See the disclaimers below.

## Disclaimers

- **Not security audited.** While security was a design priority (localhost binding, input validation, credential hygiene), this tool has not undergone formal security review. Use it on networks you control.
- **Password auth only.** SSH key authentication is not supported by design. Passwords are held in memory during the session and zeroed on disconnect, but Go's garbage collector means the original string may linger in the heap. This is a known limitation of Go's memory model.
- **No reconnection.** If the SSH connection drops, you need to restart the session. There is no automatic reconnect.
- **OUI database may be stale.** The MAC vendor database is compiled into the binary. Very new device vendors may not be recognized.
- **AI-assisted development.** The majority of this codebase was written with AI assistance (Claude). It has been reviewed, tested on real hardware, and works, but treat it as you would any personal project.

## Roadmap

- [ ] Manual device entry (custom IP + port when scan finds nothing)
- [ ] Reconnect failed tunnels without restarting
- [ ] Custom port override per device
- [ ] Additional gateway types (generic Linux, OpenWrt)

## Dependencies

- [Bubbletea](https://github.com/charmbracelet/bubbletea) -- Terminal UI framework
- [Lipgloss](https://github.com/charmbracelet/lipgloss) -- TUI styling
- [Bubbles](https://github.com/charmbracelet/bubbles) -- TUI components
- [golang.org/x/crypto](https://pkg.go.dev/golang.org/x/crypto) -- SSH client
- [endobit/oui](https://github.com/endobit/oui) -- MAC vendor lookup

## License

[MIT](LICENSE)
