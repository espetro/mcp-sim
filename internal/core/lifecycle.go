package core

import (
	"context"
	"time"

	"github.com/espetro/mcp-sim/pkg/contract"
)

// Lifecycle orchestrates boot/shutdown of devices with state management.
type Lifecycle struct {
	registry *Registry
}

// NewLifecycle creates a new lifecycle orchestrator.
func NewLifecycle(registry *Registry) *Lifecycle {
	return &Lifecycle{registry: registry}
}

// BootDevice boots a device, handling already-running and not-found errors.
func (l *Lifecycle) BootDevice(ctx context.Context, platformName, target string, opts contract.StartOpts) (contract.Device, error) {
	p, ok := l.registry.PlatformByName(platformName)
	if !ok {
		return contract.Device{}, &ToolError{Code: contract.ErrUnsupportedPlatform, Msg: "platform not found: " + platformName}
	}

	state, err := p.State(ctx, target)
	if err != nil {
		return contract.Device{}, err
	}

	if state == contract.DeviceStateRunning {
		return contract.Device{}, &ToolError{Code: contract.ErrAlreadyRunning, Msg: "device already running: " + target}
	}

	dev, err := p.Start(ctx, target, opts)
	if err != nil {
		return contract.Device{}, err
	}

	// Wait for device to become ready. Default 120s accommodates slower
	// hardware (Android emulator cold-boot, iOS Simulator first launch).
	timeout := opts.Timeout
	if timeout == 0 {
		timeout = 120 * time.Second
	}
	if err := p.AwaitReady(ctx, target, timeout); err != nil {
		return dev, &ToolError{Code: contract.ErrTimeout, Msg: "device did not become ready: " + target}
	}

	return dev, nil
}

// StopDevice stops a device.
func (l *Lifecycle) StopDevice(ctx context.Context, platformName, target string) error {
	p, ok := l.registry.PlatformByName(platformName)
	if !ok {
		return &ToolError{Code: contract.ErrUnsupportedPlatform, Msg: "platform not found: " + platformName}
	}

	state, err := p.State(ctx, target)
	if err != nil {
		return err
	}

	if state == contract.DeviceStateStopped {
		return &ToolError{Code: contract.ErrNotRunning, Msg: "device not running: " + target}
	}

	return p.Stop(ctx, target)
}

// WipeDevice wipes a device.
func (l *Lifecycle) WipeDevice(ctx context.Context, platformName, target string) error {
	p, ok := l.registry.PlatformByName(platformName)
	if !ok {
		return &ToolError{Code: contract.ErrUnsupportedPlatform, Msg: "platform not found: " + platformName}
	}

	state, err := p.State(ctx, target)
	if err != nil {
		return err
	}

	if state == contract.DeviceStateRunning {
		if err := p.Stop(ctx, target); err != nil {
			return err
		}
	}

	return p.Wipe(ctx, target)
}

// OpenURL opens a deep link on a device.
func (l *Lifecycle) OpenURL(ctx context.Context, platformName, target, url string) error {
	p, ok := l.registry.PlatformByName(platformName)
	if !ok {
		return &ToolError{Code: contract.ErrUnsupportedPlatform, Msg: "platform not found: " + platformName}
	}
	return p.OpenURL(ctx, target, url)
}
