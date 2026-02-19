package ssh

import (
	"context"
	"crypto/subtle"
	"fmt"
	"net"
	"sync"
	"time"

	gossh "golang.org/x/crypto/ssh"
)

// Client manages an SSH connection to a gateway device.
// It handles password authentication, host key verification,
// keepalive, and provides tunnel dialing.
type Client struct {
	conn       *gossh.Client
	gateway    string // host:port
	mu         sync.RWMutex
	connected  bool
	ctx        context.Context
	cancel     context.CancelFunc
	password   []byte
	knownHosts map[string]gossh.PublicKey
}

// NewClient creates a new SSH client with an empty known hosts store.
func NewClient() *Client {
	return &Client{
		knownHosts: make(map[string]gossh.PublicKey),
	}
}

// Connect establishes an SSH connection using password authentication.
// If hostKeyAlgos is non-nil, it restricts the host key algorithms
// (needed for Ubiquiti devices that require ssh-rsa).
func (c *Client) Connect(host, port, user, password string, hostKeyAlgos []string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.connected {
		return fmt.Errorf("ssh: already connected to %s", c.gateway)
	}

	addr := net.JoinHostPort(host, port)

	// Store password as bytes for later zeroing.
	c.password = []byte(password)

	config := &gossh.ClientConfig{
		User: user,
		Auth: []gossh.AuthMethod{
			gossh.Password(password),
		},
		HostKeyCallback: c.hostKeyCallback(host),
		Timeout:         10 * time.Second,
	}

	if len(hostKeyAlgos) > 0 {
		config.HostKeyAlgorithms = hostKeyAlgos
	}

	conn, err := gossh.Dial("tcp", addr, config)
	if err != nil {
		c.zeroPassword()
		return fmt.Errorf("ssh: connect to %s: %w", addr, err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	c.conn = conn
	c.gateway = addr
	c.connected = true
	c.ctx = ctx
	c.cancel = cancel

	return nil
}

// hostKeyCallback returns a callback that verifies host keys against
// the in-memory known hosts store. On first connect to a host, the key
// is accepted and stored. On subsequent connects, the key must match.
func (c *Client) hostKeyCallback(host string) gossh.HostKeyCallback {
	return func(hostname string, remote net.Addr, key gossh.PublicKey) error {
		stored, seen := c.knownHosts[host]
		if !seen {
			// First connection: trust on first use, store the key.
			c.knownHosts[host] = key
			fp := gossh.FingerprintSHA256(key)
			fmt.Fprintf(
				// Print to stderr so it doesn't interfere with TUI stdout.
				// In practice the TUI will capture this via a message.
				newStderrWriter(),
				"Host key for %s (%s):\n  %s\n",
				host, key.Type(), fp,
			)
			return nil
		}

		// Verify the key matches what we stored previously.
		// Use constant-time comparison to prevent timing side-channels.
		if key.Type() != stored.Type() ||
			subtle.ConstantTimeCompare(key.Marshal(), stored.Marshal()) != 1 {
			return fmt.Errorf(
				"ssh: host key mismatch for %s -- possible MITM attack (expected %s, got %s)",
				host,
				gossh.FingerprintSHA256(stored),
				gossh.FingerprintSHA256(key),
			)
		}
		return nil
	}
}

// IsConnected reports whether the client has an active SSH connection.
func (c *Client) IsConnected() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.connected
}

// ServerVersion returns the SSH banner string from the remote server.
// Returns an empty string if not connected.
func (c *Client) ServerVersion() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.conn == nil {
		return ""
	}
	return string(c.conn.ServerVersion())
}

// Close shuts down the SSH connection and zeroes the stored password.
func (c *Client) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.cancel != nil {
		c.cancel()
	}

	c.zeroPassword()
	c.connected = false

	if c.conn != nil {
		err := c.conn.Close()
		c.conn = nil
		if err != nil {
			return fmt.Errorf("ssh: close connection to %s: %w", c.gateway, err)
		}
	}

	return nil
}

// StartKeepalive sends periodic keepalive requests over the SSH connection.
// After 3 consecutive failures it marks the connection as disconnected.
// The goroutine exits when the client's context is cancelled (via Close).
// Must be called after Connect.
func (c *Client) StartKeepalive(interval time.Duration) {
	c.mu.RLock()
	if c.ctx == nil {
		c.mu.RUnlock()
		return
	}
	c.mu.RUnlock()

	go func() {
		failures := 0
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-c.ctx.Done():
				return
			case <-ticker.C:
				c.mu.RLock()
				conn := c.conn
				c.mu.RUnlock()

				if conn == nil {
					return
				}

				// SendRequest on the connection sends a global request.
				// "keepalive@openssh.com" is widely supported.
				_, _, err := conn.SendRequest("keepalive@openssh.com", true, nil)
				if err != nil {
					failures++
					if failures >= 3 {
						c.mu.Lock()
						c.connected = false
						c.mu.Unlock()
						return
					}
				} else {
					failures = 0
				}
			}
		}
	}()
}

// Dial opens a TCP connection through the SSH tunnel to the given
// network address. This is used for port forwarding.
func (c *Client) Dial(network, addr string) (net.Conn, error) {
	c.mu.RLock()
	conn := c.conn
	connected := c.connected
	c.mu.RUnlock()

	if !connected || conn == nil {
		return nil, fmt.Errorf("ssh: not connected, cannot dial %s", addr)
	}

	netConn, err := conn.Dial(network, addr)
	if err != nil {
		return nil, fmt.Errorf("ssh: dial %s through %s: %w", addr, c.gateway, err)
	}
	return netConn, nil
}

// zeroPassword overwrites the password bytes with zeros.
// Must be called with c.mu held.
func (c *Client) zeroPassword() {
	for i := range c.password {
		c.password[i] = 0
	}
	c.password = nil
}
