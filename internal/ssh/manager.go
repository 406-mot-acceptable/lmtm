package ssh

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// EventType describes what happened to a tunnel.
type EventType int

const (
	EventStarted EventType = iota
	EventActive
	EventFailed
	EventClosed
)

// String returns a human-readable event type.
func (e EventType) String() string {
	switch e {
	case EventStarted:
		return "started"
	case EventActive:
		return "active"
	case EventFailed:
		return "failed"
	case EventClosed:
		return "closed"
	default:
		return "unknown"
	}
}

// TunnelEvent is emitted by the Manager as tunnels change state.
// The TUI subscribes to these events to drive the build animation.
type TunnelEvent struct {
	Tunnel *Tunnel
	Type   EventType
}

// TunnelSpec describes a single port forward to build.
type TunnelSpec struct {
	RemoteHost string
	RemotePort int
	LocalPort  int
}

// Manager coordinates multiple tunnels on a single SSH connection.
// It provides an event channel that the TUI can consume to animate
// tunnel construction.
type Manager struct {
	client   *Client
	tunnels  []*Tunnel
	mu       sync.RWMutex
	eventCh  chan TunnelEvent
	closed   bool     // guards eventCh against send-after-close panic
	closeMu  sync.Mutex
	cancelFn context.CancelFunc // cancels BuildTunnels goroutine
	buildCtx context.Context
}

// NewManager creates a tunnel manager for the given SSH client.
// eventChSize controls the buffer size of the event channel.
func NewManager(client *Client, eventChSize int) *Manager {
	ctx, cancel := context.WithCancel(context.Background())
	return &Manager{
		client:   client,
		eventCh:  make(chan TunnelEvent, eventChSize),
		cancelFn: cancel,
		buildCtx: ctx,
	}
}

// Events returns a read-only channel of tunnel lifecycle events.
func (m *Manager) Events() <-chan TunnelEvent {
	return m.eventCh
}

// BuildTunnels creates and starts tunnels for each spec sequentially.
// It emits EventStarted before each tunnel starts, then EventActive
// or EventFailed depending on the outcome. A small delay between
// tunnels gives the TUI animation time to render each pipe.
// The build loop is cancelled if CloseAll is called concurrently.
func (m *Manager) BuildTunnels(specs []TunnelSpec) error {
	if len(specs) == 0 {
		return fmt.Errorf("tunnel: no specs provided")
	}

	var firstErr error

	for _, spec := range specs {
		// Check if we've been cancelled (CloseAll called during build).
		select {
		case <-m.buildCtx.Done():
			return fmt.Errorf("tunnel: build cancelled")
		default:
		}

		tun := NewTunnel(m.client, spec.LocalPort, spec.RemoteHost, spec.RemotePort)

		m.mu.Lock()
		m.tunnels = append(m.tunnels, tun)
		m.mu.Unlock()

		m.emit(TunnelEvent{Tunnel: tun, Type: EventStarted})

		if err := tun.Start(); err != nil {
			m.emit(TunnelEvent{Tunnel: tun, Type: EventFailed})
			if firstErr == nil {
				firstErr = err
			}
		} else {
			m.emit(TunnelEvent{Tunnel: tun, Type: EventActive})
		}

		// Small delay between tunnels for TUI animation pacing.
		time.Sleep(50 * time.Millisecond)
	}

	return firstErr
}

// Tunnels returns a snapshot of all managed tunnels.
func (m *Manager) Tunnels() []*Tunnel {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]*Tunnel, len(m.tunnels))
	copy(result, m.tunnels)
	return result
}

// CloseAll stops all tunnels, emits EventClosed for each, closes
// the event channel, and closes the underlying SSH client.
// Safe to call while BuildTunnels is running in a goroutine.
func (m *Manager) CloseAll() error {
	// Cancel any in-progress BuildTunnels goroutine first.
	m.cancelFn()

	m.mu.Lock()
	tunnels := make([]*Tunnel, len(m.tunnels))
	copy(tunnels, m.tunnels)
	m.mu.Unlock()

	var firstErr error
	for _, tun := range tunnels {
		if err := tun.Stop(); err != nil && firstErr == nil {
			firstErr = err
		}
		m.emit(TunnelEvent{Tunnel: tun, Type: EventClosed})
	}

	// Mark closed before closing the channel to prevent send-after-close panic.
	m.closeMu.Lock()
	m.closed = true
	close(m.eventCh)
	m.closeMu.Unlock()

	if err := m.client.Close(); err != nil && firstErr == nil {
		firstErr = err
	}

	return firstErr
}

// emit sends a tunnel event without blocking. If the channel buffer
// is full or the channel has been closed, the event is dropped.
func (m *Manager) emit(ev TunnelEvent) {
	m.closeMu.Lock()
	defer m.closeMu.Unlock()
	if m.closed {
		return
	}
	select {
	case m.eventCh <- ev:
	default:
	}
}
