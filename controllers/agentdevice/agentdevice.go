package agentdevice

import (
	"context"
	"fmt"
	"net/http"
	"os/exec"
	"strconv"
	"sync"
	"time"

	"github.com/espetro/mcp-sim/internal/config"
	"github.com/espetro/mcp-sim/pkg/contract"
)

// Controller implements contract.Controller for the agent-device proxy.
type Controller struct {
	port    int
	running bool
	mu      sync.Mutex
	stopCh  chan struct{}
	cmd     *exec.Cmd
}

// New creates a new agent-device controller adapter.
func New(cfg config.AgentDeviceConfig) *Controller {
	return &Controller{
		port:   cfg.ProxyPort,
		stopCh: make(chan struct{}),
	}
}

// Name returns "agentdevice".
func (c *Controller) Name() string { return "agentdevice" }

// Start launches the agent-device proxy daemon.
func (c *Controller) Start(ctx context.Context, cfg contract.StartConfig) (contract.ProxyInfo, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.running {
		return contract.ProxyInfo{
			Name:    c.Name(),
			URL:     proxyURL(cfg.Port),
			Running: true,
		}, nil
	}

	port := cfg.Port
	if port == 0 {
		port = c.port
	}

	cmd := exec.CommandContext(ctx, "agent-device", "proxy", "--port", strconv.Itoa(port))
	if err := cmd.Start(); err != nil {
		return contract.ProxyInfo{}, fmt.Errorf("starting agent-device: %w", err)
	}

	c.running = true
	c.stopCh = make(chan struct{})
	c.cmd = cmd

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

	if !c.running {
		return nil
	}

	if c.cmd != nil && c.cmd.Process != nil {
		_ = c.cmd.Process.Kill()
	}

	c.running = false
	close(c.stopCh)
	return nil
}

// Status checks if the proxy is running.
func (c *Controller) Status(ctx context.Context) (contract.ProxyInfo, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.running {
		return contract.ProxyInfo{Name: c.Name(), Running: false}, nil
	}

	// Try to hit the health endpoint.
	url := fmt.Sprintf("http://127.0.0.1:%d/healthz", c.port)
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	req, _ := http.NewRequestWithContext(ctx, "GET", url, nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil || resp.StatusCode != http.StatusOK {
		return contract.ProxyInfo{Name: c.Name(), URL: proxyURL(c.port), Running: false}, nil
	}

	return contract.ProxyInfo{
		Name:    c.Name(),
		URL:     proxyURL(c.port),
		Running: true,
	}, nil
}

func proxyURL(port int) string {
	return fmt.Sprintf("http://127.0.0.1:%d", port)
}
