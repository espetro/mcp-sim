package ios

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"time"

	"github.com/espetro/mcp-sim/internal/config"
	"github.com/espetro/mcp-sim/pkg/contract"
)

// Platform implements contract.Platform for iOS Simulators via xcrun simctl.
type Platform struct {
	developerDir string
}

// New creates a new iOS platform adapter.
func New(ctx context.Context, cfg config.IOSConfig) (*Platform, error) {
	devDir := cfg.DeveloperDir
	if devDir == "" {
		// Probe default developer dir.
		out, err := exec.CommandContext(ctx, "xcode-select", "-p").Output()
		if err != nil {
			return nil, fmt.Errorf("xcode-select -p: %w", err)
		}
		devDir = string(bytes.TrimSpace(out))
	}
	return &Platform{developerDir: devDir}, nil
}

// Name returns "ios".
func (p *Platform) Name() string { return "ios" }

func (p *Platform) xcrun(ctx context.Context, args ...string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, "xcrun", args...)
	cmd.Env = append(cmd.Env, "DEVELOPER_DIR="+p.developerDir)
	return cmd
}

// simctlJSON runs `xcrun simctl list devices --json` and parses the output.
func (p *Platform) simctlJSON(ctx context.Context) ([]byte, error) {
	cmd := p.xcrun(ctx, "simctl", "list", "devices", "--json")
	return cmd.Output()
}

// List returns all iOS simulators.
func (p *Platform) List(ctx context.Context) ([]contract.Device, error) {
	out, err := p.simctlJSON(ctx)
	if err != nil {
		return nil, fmt.Errorf("simctl list: %w", err)
	}

	var result struct {
		Devices []struct {
			UDID           string `json:"udid"`
			Name           string `json:"name"`
			DeviceTypeName string `json:"deviceTypeName"`
			State          string `json:"state"`
			RuntimeName    string `json:"runtimeName"`
		} `json:"devices"`
	}
	if err := json.Unmarshal(out, &result); err != nil {
		return nil, fmt.Errorf("parse simctl json: %w", err)
	}

	var devs []contract.Device
	for _, d := range result.Devices {
		if d.RuntimeName == "" {
			continue // skip base device definitions
		}
		devs = append(devs, contract.Device{
			ID:       d.UDID,
			Name:     d.Name,
			Platform: "ios",
			State:    parseSimState(d.State),
			Version:  d.RuntimeName,
		})
	}
	return devs, nil
}

// Start boots a simulator by UDID.
func (p *Platform) Start(ctx context.Context, target string, opts contract.StartOpts) (contract.Device, error) {
	cmd := p.xcrun(ctx, "simctl", "boot", target)
	if err := cmd.Run(); err != nil {
		return contract.Device{}, fmt.Errorf("simctl boot %s: %w", target, err)
	}
	return contract.Device{ID: target, Platform: "ios", State: contract.DeviceStateBooting}, nil
}

// Stop shuts down a simulator.
func (p *Platform) Stop(ctx context.Context, target string) error {
	cmd := p.xcrun(ctx, "simctl", "shutdown", target)
	_ = cmd.Run() // ignore error if already shut down
	return nil
}

// State returns the current state of a simulator.
func (p *Platform) State(ctx context.Context, target string) (contract.DeviceState, error) {
	out, err := p.xcrun(ctx, "simctl", "list", "devices", "booted", "--json").Output()
	if err != nil {
		return contract.DeviceStateUnknown, fmt.Errorf("simctl list booted: %w", err)
	}

	var result struct {
		Devices []struct {
			UDID  string `json:"udid"`
			State string `json:"state"`
		} `json:"devices"`
	}
	if err := json.Unmarshal(out, &result); err != nil {
		return contract.DeviceStateUnknown, fmt.Errorf("parse simctl json: %w", err)
	}

	for _, d := range result.Devices {
		if d.UDID == target {
			return parseSimState(d.State), nil
		}
	}
	return contract.DeviceStateStopped, nil
}

// AwaitReady polls until the simulator is booted.
func (p *Platform) AwaitReady(ctx context.Context, target string, timeout time.Duration) error {
	deadline, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-deadline.Done():
			return errors.New("await ready timeout")
		case <-ticker.C:
			state, err := p.State(ctx, target)
			if err == nil && state == contract.DeviceStateRunning {
				return nil
			}
		}
	}
}

// Wipe erases a simulator (must be stopped first).
func (p *Platform) Wipe(ctx context.Context, target string) error {
	_ = p.Stop(ctx, target) // ensure stopped
	cmd := p.xcrun(ctx, "simctl", "erase", target)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("simctl erase %s: %w", target, err)
	}
	return nil
}

// OpenURL opens a URL on the simulator.
func (p *Platform) OpenURL(ctx context.Context, target, url string) error {
	cmd := p.xcrun(ctx, "simctl", "openurl", target, url)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("simctl openurl %s %s: %w", target, url, err)
	}
	return nil
}

func parseSimState(s string) contract.DeviceState {
	switch s {
	case "Booted":
		return contract.DeviceStateRunning
	case "Booting":
		return contract.DeviceStateBooting
	case "Shutdown":
		return contract.DeviceStateStopped
	default:
		return contract.DeviceStateUnknown
	}
}