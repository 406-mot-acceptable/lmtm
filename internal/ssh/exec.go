package ssh

import (
	"context"
	"fmt"
	"strings"
)

// Exec runs a command on the remote gateway and returns the combined
// stdout+stderr output. It creates a new SSH session per call, which
// is cheap on a multiplexed SSH connection. The context controls
// cancellation and timeout.
func (c *Client) Exec(ctx context.Context, cmd string) (string, error) {
	c.mu.RLock()
	conn := c.conn
	connected := c.connected
	c.mu.RUnlock()

	if !connected || conn == nil {
		return "", fmt.Errorf("ssh: not connected, cannot exec %q", cmd)
	}

	session, err := conn.NewSession()
	if err != nil {
		return "", fmt.Errorf("ssh: new session for %q: %w", cmd, err)
	}
	defer session.Close()

	// Run the command in a goroutine so we can respect context cancellation.
	type result struct {
		output []byte
		err    error
	}
	ch := make(chan result, 1)

	go func() {
		out, err := session.CombinedOutput(cmd)
		ch <- result{out, err}
	}()

	select {
	case <-ctx.Done():
		// Signal the session to close, which will cause CombinedOutput
		// to return with an error in the goroutine.
		session.Close()
		return "", fmt.Errorf("ssh: exec %q: %w", cmd, ctx.Err())
	case r := <-ch:
		output := strings.TrimSpace(string(r.output))
		if r.err != nil {
			return output, fmt.Errorf("ssh: exec %q: %w", cmd, r.err)
		}
		return output, nil
	}
}
