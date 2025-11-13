# The Tunneler (Go Edition)

A blazing-fast SSH tunnel manager written in Go with a beautiful TUI. Built for managing 800+ customer sites with minimal configuration.

## Why Go Version?

- **Single Binary**: No dependencies, just run `./tunneler`
- **Fast Startup**: 10-20x faster than Python version
- **Cross-Platform**: Compile for Linux, macOS, Windows
- **Small Footprint**: ~9MB binary (vs Python + venv + deps)
- **Quick Commands**: No config file needed for common tasks

## Features

✓ **Minimal Config** - Just gateway IPs, devices computed on-demand
✓ **Quick Mode** - Instant tunnels without config file
✓ **Network Discovery** - Scan networks, detect devices with MAC vendor lookup
✓ **Smart Port Mapping** - `10.0.0.X → localhost:443X`
✓ **Multi-Subnet Support** - Scan multiple bogon subnets simultaneously
✓ **Device Type Handling** - Auto-adds Ubiquiti SSH options, MikroTik compatible
✓ **Beautiful TUI** - Bubbletea-powered interface with debug logs
✓ **Fuzzy Search** - Find sites quickly in 800+ list
✓ **Single Binary** - No virtual environments or dependencies

## Installation

### Option 1: Build from Source

```bash
cd the_tunneler_go
make deps      # Install Go dependencies
make build     # Build binary to ./bin/tunneler
```

### Option 2: Install to System

```bash
make install   # Copies to /usr/local/bin/tunneler
```

### Option 3: Build Optimized Release

```bash
make release   # Smaller binary with optimizations
```

## Quick Start (No Config File Needed!)

### Tunnel to First 10 Devices

```bash
./bin/tunneler quick --gateway 102.217.230.33 --first-10
```

This creates tunnels for `10.0.0.2-11` → `localhost:4432-4441`

### Tunnel to Custom Range

```bash
./bin/tunneler quick --gateway 102.217.230.33 --range-start 2 --range-end 20
```

Tunnels `10.0.0.2-20` through the gateway.

### Different Gateway Type

```bash
./bin/tunneler quick --gateway 198.51.100.45 --type mikrotik --user datonetfullaccess --first-10
```

## Configuration (For 800+ Sites)

Create `tunneler.yaml`:

```yaml
defaults:
  username: "dato"
  subnet: "10.0.0"
  password_prompt: true

sites:
  # Minimal - just gateway info
  - name: "Client Site Alpha"
    gateway: "102.217.230.33"
    type: "ubiquiti"
    favorite: true

  # With custom username
  - name: "Client Site Beta"
    gateway: "198.51.100.45"
    type: "mikrotik"
    username: "datonetfullaccess"

  # With custom device range
  - name: "Client Site Gamma (Large)"
    gateway: "203.0.113.50"
    type: "ubiquiti"
    device_range:
      start: 2
      end: 30  # Tunnel 10.0.0.2-30
```

**For 800+ sites**: Just list gateway IPs with minimal info. Devices are computed dynamically.

## Interactive TUI Mode

```bash
./bin/tunneler
```

Features:
- **Fuzzy search** - Type `/` then search for site names
- **Favorites first** - Mark important sites with `favorite: true`
- **Password prompt** - Enter once per session
- **Real-time status** - See connection states: ✓ active, ⋯ connecting, ✗ failed

Keyboard shortcuts:
- `↑/↓` - Navigate sites
- `Enter` - Connect to selected site
- `/` - Filter/search sites
- `l` - Toggle debug log view
- `d` - Disconnect all
- `q` - Quit

## Network Discovery & Smart Scanning

The Tunneler can discover active devices on customer networks before creating tunnels:

### Discovery Presets

```yaml
presets:
  discover:
    name: "Network Discovery"
    type: "scan"
    scan_method: "arp"  # Fast: uses ARP cache
    scan_ports: [22, 80, 443, 554, 8080, 8443]
    auto_tunnel: false  # Show selection UI

  multi-subnet-scan:
    name: "Multi-Subnet Scan"
    type: "scan"
    scan_method: "arp"
    scan_ports: [22, 80, 443, 554]
    subnets: ["10.0.0", "192.168.1", "172.16.0"]  # Multiple bogon subnets
    auto_tunnel: false
```

### Scan Methods

- **ARP** (Fastest) - Reads ARP cache for instant discovery
  - Works on MikroTik (`/ip arp print`) and Linux (`ip neigh show`)
  - No network traffic generated

- **Ping** (Medium) - Active ping sweep
  - Sends ICMP to all IPs in subnet
  - Works on both gateway types

- **Nmap** (Comprehensive) - Full network scan
  - Requires nmap installed on gateway
  - Most accurate but slowest

### Features

✓ **MAC Vendor Lookup** - Identifies device manufacturers (Hikvision, Dahua, Axis cameras)
✓ **Device Type Detection** - Smart guessing: Camera, NVR, Network Device, Web Server
✓ **Interactive Selection** - Choose which devices to tunnel to
✓ **Port Customization** - Override detected ports per device
✓ **Multi-Subnet Support** - Scan multiple network ranges simultaneously
✓ **Client-Side Fallback** - Port scanning works even without netcat/bash on gateway

### Interactive Device Selection

After scanning, use the TUI to select devices:

- `Space` - Toggle device selection
- `a` - Select all
- `n` - Select none
- `p` - Edit port for current device
- `Enter` - Connect to selected devices
- `Esc` - Cancel

The selection shows:
- IP Address
- Open Ports
- Vendor (from MAC address OUI lookup)
- Device Type (guessed from ports + vendor)

### Per-Site Subnet Override

```yaml
sites:
  - name: "Custom Subnet Site"
    gateway: "203.0.113.75"
    type: "ubiquiti"
    subnet: "192.168.1"  # Use 192.168.1.0/24 instead of default 10.0.0
```

### MikroTik Support

Full compatibility with MikroTik RouterOS:
- Gateway-specific ARP commands (`/ip arp print`)
- RouterOS-aware ping sweep
- Client-side port scanning (bypasses missing netcat/bash)
- Tested on RouterOS v6 and v7

## Usage Scenarios

### Scenario 1: Emergency Access (No Config)

```bash
tunneler quick --gateway 102.217.230.33 --first-10
# Access devices immediately at localhost:4432-4441
```

### Scenario 2: Managing 800+ Sites

```yaml
# tunneler.yaml - Just list gateways
sites:
  - {name: "Site 001", gateway: "203.0.113.1", type: "ubiquiti"}
  - {name: "Site 002", gateway: "203.0.113.2", type: "ubiquiti"}
  # ... 798 more sites
```

Launch TUI, use fuzzy search (`/site-042`) to find and connect.

### Scenario 3: Large Internal Network

```bash
# Tunnel to 30 devices at once
tunneler quick --gateway 102.217.230.33 --range-start 2 --range-end 31
```

## Port Mapping

Same smart mapping as Python version:

- `10.0.0.2:443` → `localhost:4432`
- `10.0.0.3:443` → `localhost:4433`
- `10.0.0.X:443` → `localhost:443X`

Pattern: Last octet + 4430 = local port

## Command Reference

### Quick Command

```bash
# Basic usage
tunneler quick --gateway <IP> --first-10

# All options
tunneler quick \
  --gateway <IP> \           # Gateway IP (required)
  --user <username> \        # SSH username (default: dato)
  --type <type> \            # ubiquiti or mikrotik (default: ubiquiti)
  --subnet <subnet> \        # Device subnet (default: 10.0.0)
  --first-10 \               # Tunnel 10.0.0.2-11
  --range-start <n> \        # Custom range start
  --range-end <n>            # Custom range end
```

### TUI Mode

```bash
# With config file
tunneler --config ./tunneler.yaml

# Default locations checked:
# - ./tunneler.yaml
# - ~/.config/tunneler/config.yaml
```

## Building

### Requirements

- Go 1.21+ (automatically upgraded if needed)
- Make (optional, can use `go build` directly)

### Build Commands

```bash
make build     # Debug build → ./bin/tunneler
make release   # Optimized build (smaller)
make clean     # Remove build artifacts
make install   # Install to /usr/local/bin
make help      # Show all targets
```

### Manual Build

```bash
go build -o tunneler cmd/tunneler/main.go
```

## Project Structure

```
the_tunneler_go/
├── cmd/tunneler/
│   └── main.go              # Entry point
├── internal/
│   ├── cli/
│   │   ├── root.go          # Cobra root command
│   │   ├── quick.go         # Quick tunnel command
│   │   └── tui.go           # TUI launcher
│   ├── config/
│   │   └── config.go        # YAML config + device generation
│   ├── scanner/
│   │   └── scanner.go       # Network discovery engine
│   ├── ssh/
│   │   ├── manager.go       # Multi-site tunnel manager
│   │   ├── tunnel.go        # Single SSH connection
│   │   └── commands.go      # Remote command execution + parsing
│   └── tui/
│       ├── model.go         # Bubbletea TUI
│       ├── device_selector.go  # Device selection UI
│       └── logger.go        # Debug logging
├── configs/
│   └── example.yaml         # Example configuration
├── Makefile                 # Build targets
└── go.mod                   # Go module
```

## Comparison: Go vs Python Version

| Feature | Go | Python |
|---------|-----|--------|
| **Startup Time** | <100ms | 500-1000ms |
| **Binary Size** | 8.8MB single file | 30MB+ (venv + deps) |
| **Memory Usage** | ~15MB | ~50MB |
| **Deployment** | Copy binary | venv + pip install |
| **Cross-Platform** | Compile once per platform | Requires Python install |
| **Quick Mode** | Built-in CLI flags | Requires editing config |
| **TUI Framework** | Bubbletea | Textual |
| **SSH Library** | golang.org/x/crypto/ssh | AsyncSSH |

**Both versions work great!** Use whichever fits your workflow.

## Troubleshooting

### "Command not found"

```bash
# If using ./bin/tunneler
export PATH=$PATH:$(pwd)/bin

# Or install to system
make install
```

### "Permission denied"

```bash
chmod +x bin/tunneler
```

### "Failed to connect"

- Check gateway IP is reachable
- Verify SSH is enabled on gateway
- For Ubiquiti: ensure `type: "ubiquiti"` in config
- Check username (dato vs datonetfullaccess)

### "Port already in use"

Another tunnel is using that local port. Disconnect existing tunnels first.

## Tips & Best Practices

1. **Use quick mode** for one-off access - no config needed
2. **Keep config minimal** - just gateway IPs, let ranges compute
3. **Use favorites** for frequent sites - they appear first in TUI
4. **Fuzzy search** is your friend with 800+ sites
5. **Single binary** makes deployment to multiple machines trivial

## Development

### Run Without Building

```bash
go run cmd/tunneler/main.go quick --gateway 102.217.230.33 --first-10
```

### Add Dependencies

```bash
go get github.com/some/package
go mod tidy
```

## License

This project is provided as-is for managing network infrastructure.

## Credits

Built with:
- [Cobra](https://github.com/spf13/cobra) - CLI framework
- [Bubbletea](https://github.com/charmbracelet/bubbletea) - TUI framework
- [Bubbles](https://github.com/charmbracelet/bubbles) - TUI components
- [Lipgloss](https://github.com/charmbracelet/lipgloss) - Terminal styling
- [golang.org/x/crypto/ssh](https://pkg.go.dev/golang.org/x/crypto/ssh) - SSH library
- [OUI](https://github.com/endobit/oui) - MAC vendor lookup database

---

**Quick Reference:**

```bash
# Quick tunnel (no config)
tunneler quick --gateway 102.217.230.33 --first-10

# Interactive TUI
tunneler

# Build
make build

# Help
tunneler --help
tunneler quick --help
```
