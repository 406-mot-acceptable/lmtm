package browser

import (
	"fmt"
	"os/exec"
	"runtime"

	"github.com/jaco/tunneler/internal/ssh"
)

// Opener handles opening URLs in browsers
type Opener struct {
	browserCmd string
}

// NewOpener creates a new browser opener
func NewOpener() *Opener {
	return &Opener{
		browserCmd: detectBrowser(),
	}
}

// detectBrowser detects the available browser command
func detectBrowser() string {
	// Prefer Firefox for better control
	browsers := []string{"firefox", "firefox-esr", "google-chrome", "chromium", "brave"}

	for _, browser := range browsers {
		if _, err := exec.LookPath(browser); err == nil {
			return browser
		}
	}

	// Fallback to OS default
	switch runtime.GOOS {
	case "darwin":
		return "open"
	case "windows":
		return "start"
	default:
		return "xdg-open"
	}
}

// OpenTunnels opens browser tabs for all active tunnels
func (o *Opener) OpenTunnels(tunnels map[string][]*ssh.TunnelInfo, protocol string) error {
	urls := []string{}

	for _, siteTunnels := range tunnels {
		for _, tunnel := range siteTunnels {
			if tunnel.Status == ssh.StatusActive {
				url := o.buildURL(tunnel, protocol)
				urls = append(urls, url)
			}
		}
	}

	if len(urls) == 0 {
		return fmt.Errorf("no active tunnels to open")
	}

	return o.OpenURLs(urls)
}

// OpenURLs opens multiple URLs in browser tabs
func (o *Opener) OpenURLs(urls []string) error {
	if len(urls) == 0 {
		return nil
	}

	// Firefox supports opening multiple URLs at once
	if o.browserCmd == "firefox" || o.browserCmd == "firefox-esr" {
		args := []string{}
		for _, url := range urls {
			args = append(args, url)
		}
		cmd := exec.Command(o.browserCmd, args...)
		return cmd.Start()
	}

	// For other browsers, open each URL separately
	for _, url := range urls {
		cmd := exec.Command(o.browserCmd, url)
		if err := cmd.Start(); err != nil {
			return err
		}
	}

	return nil
}

// buildURL constructs the URL for a tunnel
func (o *Opener) buildURL(tunnel *ssh.TunnelInfo, protocol string) string {
	// Auto-detect protocol based on port if not specified
	if protocol == "" {
		switch tunnel.DevicePort {
		case 443, 8443:
			protocol = "https"
		case 554:
			// RTSP streams need special handling - just show the address
			return fmt.Sprintf("rtsp://localhost:%d", tunnel.LocalPort)
		default:
			protocol = "http"
		}
	}

	return fmt.Sprintf("%s://localhost:%d", protocol, tunnel.LocalPort)
}

// GetBrowserCommand returns the detected browser command
func (o *Opener) GetBrowserCommand() string {
	return o.browserCmd
}
