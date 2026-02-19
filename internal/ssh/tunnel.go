package ssh

import (
	"context"
	"fmt"
	"io"
	"net"
	"sync/atomic"
	"time"
)

// TunnelStatus represents the current state of a tunnel.
type TunnelStatus int

const (
	StatusDisconnected TunnelStatus = iota
	StatusConnecting
	StatusActive
	StatusFailed
)

// String returns a human-readable tunnel status.
func (s TunnelStatus) String() string {
	switch s {
	case StatusDisconnected:
		return "disconnected"
	case StatusConnecting:
		return "connecting"
	case StatusActive:
		return "active"
	case StatusFailed:
		return "failed"
	default:
		return "unknown"
	}
}

// Tunnel manages a single local-to-remote port forward over an SSH connection.
// It listens on 127.0.0.1:LocalPort and forwards accepted connections through
// the SSH client to RemoteHost:RemotePort.
type Tunnel struct {
	LocalPort  int
	RemoteHost string
	RemotePort int
	Status     TunnelStatus
	Error      error

	listener  net.Listener
	client    *Client
	ctx       context.Context
	cancel    context.CancelFunc
	connCount int64 // atomic: number of active forwarded connections
}

// NewTunnel creates a tunnel that will forward from localhost:localPort
// through the SSH client to remoteHost:remotePort.
func NewTunnel(client *Client, localPort int, remoteHost string, remotePort int) *Tunnel {
	ctx, cancel := context.WithCancel(context.Background())
	return &Tunnel{
		LocalPort:  localPort,
		RemoteHost: remoteHost,
		RemotePort: remotePort,
		Status:     StatusDisconnected,
		client:     client,
		ctx:        ctx,
		cancel:     cancel,
	}
}

// Start begins listening on 127.0.0.1:LocalPort and forwarding connections.
// It binds exclusively to loopback to prevent external access.
func (t *Tunnel) Start() error {
	t.Status = StatusConnecting

	listenAddr := fmt.Sprintf("127.0.0.1:%d", t.LocalPort)
	ln, err := net.Listen("tcp", listenAddr)
	if err != nil {
		t.Status = StatusFailed
		t.Error = fmt.Errorf("tunnel: listen on %s: %w", listenAddr, err)
		return t.Error
	}
	t.listener = ln
	t.Status = StatusActive

	// Accept loop runs in background.
	go t.acceptLoop()

	return nil
}

// acceptLoop accepts incoming connections on the local listener and
// forwards each one through the SSH tunnel.
func (t *Tunnel) acceptLoop() {
	consecutiveErrors := 0
	for {
		conn, err := t.listener.Accept()
		if err != nil {
			// Listener closed (via Stop) -- exit cleanly.
			select {
			case <-t.ctx.Done():
				return
			default:
			}
			// Backoff on persistent accept errors to avoid tight spin.
			consecutiveErrors++
			if consecutiveErrors >= 10 {
				t.Status = StatusFailed
				t.Error = fmt.Errorf("tunnel: too many accept errors on port %d: %w", t.LocalPort, err)
				return
			}
			time.Sleep(time.Duration(consecutiveErrors) * 50 * time.Millisecond)
			continue
		}
		consecutiveErrors = 0
		go t.forward(conn)
	}
}

// forward connects the local connection to the remote host through the
// SSH tunnel and copies data bidirectionally.
func (t *Tunnel) forward(local net.Conn) {
	atomic.AddInt64(&t.connCount, 1)
	defer atomic.AddInt64(&t.connCount, -1)
	defer local.Close()

	remoteAddr := fmt.Sprintf("%s:%d", t.RemoteHost, t.RemotePort)
	remote, err := t.client.Dial("tcp", remoteAddr)
	if err != nil {
		return
	}
	defer remote.Close()

	// Bidirectional copy: two goroutines, done when either direction finishes.
	// Buffer of 2 so neither goroutine blocks on send after the function returns.
	done := make(chan struct{}, 2)

	go func() {
		io.Copy(remote, local)
		done <- struct{}{}
	}()

	go func() {
		io.Copy(local, remote)
		done <- struct{}{}
	}()

	// Wait for either direction to finish, or context cancellation.
	// On context cancel, deferred Close calls will unblock the io.Copy goroutines.
	select {
	case <-done:
	case <-t.ctx.Done():
	}
}

// Stop cancels the tunnel, closes the listener, and waits up to 5 seconds
// for active forwarded connections to drain.
func (t *Tunnel) Stop() error {
	t.cancel()

	if t.listener != nil {
		t.listener.Close()
	}

	// Wait for active connections to drain, up to 5 seconds.
	deadline := time.After(5 * time.Second)
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()

	for {
		if atomic.LoadInt64(&t.connCount) == 0 {
			break
		}
		select {
		case <-deadline:
			// Timed out waiting for connections to drain.
			t.Status = StatusDisconnected
			return fmt.Errorf("tunnel: %d connections still active after 5s drain timeout on port %d",
				atomic.LoadInt64(&t.connCount), t.LocalPort)
		case <-ticker.C:
			continue
		}
	}

	t.Status = StatusDisconnected
	return nil
}

// ActiveConnections returns the number of currently active forwarded connections.
func (t *Tunnel) ActiveConnections() int64 {
	return atomic.LoadInt64(&t.connCount)
}
