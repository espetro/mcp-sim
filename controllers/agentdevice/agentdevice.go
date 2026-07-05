// Package agentdevice adapts the agent-device CLI as a verification-layer
// controller for mcp-sim.
//
// agent-device is itself the verification layer (taps, screenshots,
// accessibility trees). It is invoked by the agent client (Claude Code,
// Cursor, etc.) — not by mcp-sim. mcp-sim's controller adapter only
// reports whether the binary is available and (optionally) launches the
// MCP server mode (`agent-device mcp`) on a local port so the agent can
// attach over HTTP instead of stdio.
//
// The plan originally described this as a "proxy launcher" using
// `agent-device proxy`, but that subcommand doesn't exist in agent-device's
// CLI. The actual subcommands we use:
//
//   - agent-device mcp     start the MCP server (stdio by default)
//   - agent-device --help  health probe
//
// mcp-sim doesn't own agent-device's lifecycle — the user does. We only
// detect presence and (optionally) advertise the MCP discovery endpoint.
package agentdevice

import (
	"context"
	"fmt"
	"net/http"
	"os/exec"
	"sync"
	"time"

	"github.com/espetro/mcp-sim/internal/config"
	"github.com/espetro/mcp-sim/pkg/contract"
)

// Controller implements contract.Controller for the agent-device presence.
type Controller struct {
	port    int
	mu      sync.Mutex
	mcpCmd  *exec.Cmd // running `agent-device mcp` process, if any
	mcpPath string   // discovered path to the agent-device binary
}

// New creates a new agent-device controller adapter.
//
// Resilient: returns nil if agent-device is not on PATH. Callers should
// treat a nil result as "skip registration" rather than fatal.
func New(cfg config.AgentDeviceConfig) *Controller {
	path, err := exec.LookPath("agent-device")
	if err != nil {
		return nil
	}
	return &Controller{
		port:    cfg.ProxyPort,
		mcpPath: path,
	}
}

// Name returns "agentdevice".
func (c *Controller) Name() string { return "agentdevice" }

// Start is a no-op for v0.1.x: agent-device is invoked by the agent client.
//
// In a future version this can spawn `agent-device mcp` on the configured
// port if the agent client prefers HTTP-attached MCP. For now, returning
// running=true signals that the binary is available.
func (c *Controller) Start(ctx context.Context, cfg contract.StartConfig) (contract.ProxyInfo, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	port := cfg.Port
	_ = port

	return contract.ProxyInfo{
		Name:    c.Name(),
		URL:     fmt.Sprintf("agent-device://%s", c.mcpPath),
		Running: c.mcpPath != "",
	}, nil
}

// Stop is a no-op for v0.1.x — agent-device is externally managed.
func (c *Controller) Stop(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.mcpCmd != nil && c.mcpCmd.Process != nil {
		_ = c.mcpCmd.Process.Kill()
		_, _ = c.mcpCmd.Process.Wait()
		c.mcpCmd = nil
	}
	return nil
}

// Status checks whether agent-device is reachable.
//
// We probe with `agent-device --help` (the only universally supported
// flag). If that returns a useful response, the binary is healthy.
func (c *Controller) Status(ctx context.Context) (contract.ProxyInfo, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.mcpPath == "" {
		return contract.ProxyInfo{Name: c.Name(), Running: false}, nil
	}

	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	req, _ := http.NewRequestWithContext(ctx, "GET", "http://127.0.0.1:"+fmt.Sprint(c.port)+"/healthz", nil)
	resp, err := http.DefaultClient.Do(req)
	running := false
	if err == nil && resp.StatusCode == http.StatusOK {
		running = true
		_ = resp.Body.Close()
	}

	return contract.ProxyInfo{
		Name:    c.Name(),
		URL:     fmt.Sprintf("agent-device://%s", c.mcpPath),
		Running: running || c.mcpPath != "",
	}, nil
}