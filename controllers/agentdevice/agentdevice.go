package agentdevice

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/espetro/mcp-sim/internal/config"
	"github.com/espetro/mcp-sim/pkg/contract"
)

// Controller implements contract.Controller for the agent-device proxy daemon.
//
// agent-device v0.15+ ships a `proxy` subcommand that exposes the local
// daemon over HTTP for tunnel-friendly access (cloudflared, ngrok, Tailscale).
// We spawn it as a detached child process and track the PID for guaranteed
// cleanup on Stop.
//
// Lifecycle (mcp-sim owned):
//   - Start(): spawn `agent-device proxy --port N` detached from request ctx
//   - Stop():  SIGTERM the process group, fall back to SIGKILL
//   - Status(): HTTP probe against the proxy /health endpoint
//
// mcp-sim does NOT call `agent-device mcp` (stdio MCP) or `agent-device
// connect` (remote daemon connection). Those are invoked by the agent
// client against the proxy URL we publish.
type Controller struct {
	port     int
	authTok  string // optional --daemon-auth-token
	mu       sync.Mutex
	cmd      *exec.Cmd
	proc     *os.Process
}

// New creates a new agent-device controller adapter.
//
// Resilient: returns nil if agent-device is not on PATH. Callers should
// treat a nil result as "skip registration" rather than fatal.
func New(cfg config.AgentDeviceConfig) *Controller {
	if _, err := exec.LookPath("agent-device"); err != nil {
		return nil
	}
	return &Controller{
		port:    cfg.ProxyPort,
		authTok: os.Getenv("MCPSIM_AGENT_DEVICE_AUTH_TOKEN"),
	}
}

// Name returns "agentdevice".
func (c *Controller) Name() string { return "agentdevice" }

// Start launches `agent-device proxy` on the configured port.
//
// Spawned detached from the request ctx so the proxy survives the
// originating MCP request. Stdout/stderr redirected to io.Discard to
// avoid SIGPIPE on backgrounded server stdio.
func (c *Controller) Start(ctx context.Context, cfg contract.StartConfig) (contract.ProxyInfo, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.proc != nil {
		// Already running — return existing info.
		return contract.ProxyInfo{
			Name:    c.Name(),
			URL:     proxyURL(c.port),
			Running: true,
		}, nil
	}

	port := cfg.Port
	if port == 0 {
		port = c.port
	}

	args := []string{"proxy", "--port", strconv.Itoa(port), "--host", "127.0.0.1"}
	if c.authTok != "" {
		args = append(args, "--daemon-auth-token", c.authTok)
	}

	// Detached from request ctx: the proxy must outlive the MCP tool
	// call. cmd is killed only by Stop() or by the server's
	// ShutdownAll().
	cmd := exec.CommandContext(context.Background(), "agent-device", args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Stdin = nil
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	if err := cmd.Start(); err != nil {
		return contract.ProxyInfo{}, fmt.Errorf("starting agent-device proxy: %w", err)
	}

	c.cmd = cmd
	c.proc = cmd.Process

	return contract.ProxyInfo{
		Name:    c.Name(),
		URL:     proxyURL(port),
		Running: true,
	}, nil
}

// Stop terminates the agent-device proxy daemon.
func (c *Controller) Stop(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.proc == nil {
		return nil
	}

	proc := c.proc
	c.cmd = nil
	c.proc = nil

	// Negative PID = kill the entire process group (Setpgid:true).
	if err := syscall.Kill(-proc.Pid, syscall.SIGTERM); err != nil {
		// Process may already be gone; fall through to SIGKILL.
		_ = syscall.Kill(proc.Pid, syscall.SIGKILL)
	}

	// Best-effort wait so we don't leak a zombie.
	done := make(chan struct{})
	go func() { _, _ = proc.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		_ = syscall.Kill(proc.Pid, syscall.SIGKILL)
	}
	return nil
}

// Status checks whether the proxy is reachable.
//
// Probes the proxy's /health endpoint. agent-device proxy exposes
// /health unauthenticated for reachability checks, so this works
// without a bearer token.
func (c *Controller) Status(ctx context.Context) (contract.ProxyInfo, error) {
	c.mu.Lock()
	port := c.port
	running := c.proc != nil
	c.mu.Unlock()

	url := proxyURL(port) + "/health"
	rctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	req, _ := http.NewRequestWithContext(rctx, "GET", url, nil)
	resp, err := http.DefaultClient.Do(req)
	if err == nil {
		defer func() { _ = resp.Body.Close() }()
		if resp.StatusCode == http.StatusOK {
			return contract.ProxyInfo{
				Name:    c.Name(),
				URL:     proxyURL(port),
				Running: true,
			}, nil
		}
	}

	return contract.ProxyInfo{
		Name:    c.Name(),
		URL:     proxyURL(port),
		Running: running,
	}, nil
}

// HelpInfo returns human-readable usage info shown by `start_controller`
// tool descriptions. Surfaces the auth-token env var so users know
// where to set it for non-loopback tunnels.
func (c *Controller) HelpInfo() string {
	var b strings.Builder
	b.WriteString("Spawns `agent-device proxy --port N` as a detached child.\n")
	b.WriteString("For tunnel access (cloudflared, ngrok, Tailscale), set\n")
	b.WriteString("MCPSIM_AGENT_DEVICE_AUTH_TOKEN before starting mcp-sim; the\n")
	b.WriteString("token is forwarded as `--daemon-auth-token`.\n")
	b.WriteString("The proxy exposes /health (unauthenticated), /rpc, /upload,\n")
	b.WriteString("/artifacts, and the same routes under /agent-device/*.\n")
	_ = errors.New("unused") // keep the import
	return b.String()
}

func proxyURL(port int) string {
	return fmt.Sprintf("http://127.0.0.1:%d", port)
}