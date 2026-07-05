package core

import (
	"context"
	"time"

	"github.com/espetro/mcp-sim/pkg/contract"
)

// Lifecycle orchestrates boot/shutdown of devices with state management.
type Lifecycle struct {
	registry *Registry
	sessions *Manager
}

// NewLifecycle creates a new lifecycle orchestrator.
func NewLifecycle(registry *Registry, sessions *Manager) *Lifecycle {
	return &Lifecycle{registry: registry, sessions: sessions}
}

// BootDevice boots a device, handling already-running and not-found errors.
func (l *Lifecycle) BootDevice(ctx context.Context, platformName, target, sessionID string, opts contract.StartOpts) (dev contract.Device, err error) {
	p, ok := l.registry.PlatformByName(platformName)
	if !ok {
		return contract.Device{}, &ToolError{Code: contract.ErrUnsupportedPlatform, Msg: "platform not found: " + platformName}
	}

	if err := l.sessions.Reserve(ctx, platformName, target, sessionID); err != nil {
		return contract.Device{}, err
	}
	defer func() {
		if err != nil {
			l.sessions.Release(platformName, target)
		}
	}()

	state, err := p.State(ctx, target)
	if err != nil {
		return contract.Device{}, err
	}

	if state == contract.DeviceStateRunning {
		return contract.Device{}, &ToolError{Code: contract.ErrAlreadyRunning, Msg: "device already running: " + target}
	}

	dev, err = p.Start(ctx, target, opts)
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

	l.sessions.RecordActivity(platformName, target)
	return dev, nil
}

// StopDevice stops a device.
func (l *Lifecycle) StopDevice(ctx context.Context, platformName, target, sessionID string) error {
	p, ok := l.registry.PlatformByName(platformName)
	if !ok {
		return &ToolError{Code: contract.ErrUnsupportedPlatform, Msg: "platform not found: " + platformName}
	}

	if err := l.sessions.CheckAccess(platformName, target, sessionID); err != nil {
		return err
	}

	state, err := p.State(ctx, target)
	if err != nil {
		return err
	}

	if state == contract.DeviceStateStopped {
		return &ToolError{Code: contract.ErrNotRunning, Msg: "device not running: " + target}
	}

	if err := p.Stop(ctx, target); err != nil {
		return err
	}

	l.sessions.Release(platformName, target)
	return nil
}

// WipeDevice wipes a device.
func (l *Lifecycle) WipeDevice(ctx context.Context, platformName, target, sessionID string) error {
	p, ok := l.registry.PlatformByName(platformName)
	if !ok {
		return &ToolError{Code: contract.ErrUnsupportedPlatform, Msg: "platform not found: " + platformName}
	}

	if err := l.sessions.CheckAccess(platformName, target, sessionID); err != nil {
		return err
	}
	defer l.sessions.Release(platformName, target)

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
func (l *Lifecycle) OpenURL(ctx context.Context, platformName, target, sessionID, url string) error {
	p, ok := l.registry.PlatformByName(platformName)
	if !ok {
		return &ToolError{Code: contract.ErrUnsupportedPlatform, Msg: "platform not found: " + platformName}
	}

	if err := l.sessions.CheckAccess(platformName, target, sessionID); err != nil {
		return err
	}
	l.sessions.RecordActivity(platformName, target)
	return p.OpenURL(ctx, target, url)
}
