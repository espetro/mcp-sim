package core

import (
	"context"
	"fmt"
	"time"

	"github.com/espetro/mcp-sim/pkg/contract"
)

// ToolError wraps an error with a stable MCP error code.
type ToolError struct {
	Code string
	Msg  string
}

func (e *ToolError) Error() string {
	return e.Msg
}

// ListDevices returns all devices across all platforms.
func ListDevices(ctx context.Context, registry *Registry, sessions *Manager) ([]contract.Device, error) {
	var result []contract.Device
	for name, p := range registry.AllPlatforms() {
		devs, err := p.List(ctx)
		if err != nil {
			return nil, fmt.Errorf("listing devices on %s: %w", name, err)
		}
		for i := range devs {
			if owner, ok := sessions.Owner(name, devs[i].ID); ok {
				devs[i].OwnerSession = owner
			}
		}
		result = append(result, devs...)
	}
	return result, nil
}

// GetDeviceState returns the state of a specific device.
func GetDeviceState(ctx context.Context, registry *Registry, sessions *Manager, platformName, target, sessionID string) (contract.DeviceState, error) {
	p, ok := registry.PlatformByName(platformName)
	if !ok {
		return contract.DeviceStateUnknown, &ToolError{Code: contract.ErrUnsupportedPlatform, Msg: "platform not found: " + platformName}
	}
	if err := sessions.CheckAccess(platformName, target, sessionID); err != nil {
		return contract.DeviceStateUnknown, err
	}
	state, err := p.State(ctx, target)
	if err != nil {
		return contract.DeviceStateUnknown, err
	}
	sessions.RecordActivity(platformName, target)
	return state, nil
}

// StartController starts a controller.
func StartController(ctx context.Context, registry *Registry, name string, cfg contract.StartConfig) (contract.ProxyInfo, error) {
	c, ok := registry.ControllerByName(name)
	if !ok {
		return contract.ProxyInfo{}, &ToolError{Code: contract.ErrUnsupportedController, Msg: "controller not found: " + name}
	}
	return c.Start(ctx, cfg)
}

// StopController stops a controller.
func StopController(ctx context.Context, registry *Registry, name string) error {
	c, ok := registry.ControllerByName(name)
	if !ok {
		return &ToolError{Code: contract.ErrUnsupportedController, Msg: "controller not found: " + name}
	}
	return c.Stop(ctx)
}

// ControllerStatus returns controller status.
func ControllerStatus(ctx context.Context, registry *Registry, name string) (contract.ProxyInfo, error) {
	c, ok := registry.ControllerByName(name)
	if !ok {
		return contract.ProxyInfo{}, &ToolError{Code: contract.ErrUnsupportedController, Msg: "controller not found: " + name}
	}
	return c.Status(ctx)
}

// AwaitDeviceReady waits for a device to become ready.
func AwaitDeviceReady(ctx context.Context, registry *Registry, sessions *Manager, platformName, target, sessionID string, timeout time.Duration) error {
	p, ok := registry.PlatformByName(platformName)
	if !ok {
		return &ToolError{Code: contract.ErrUnsupportedPlatform, Msg: "platform not found: " + platformName}
	}
	if err := sessions.CheckAccess(platformName, target, sessionID); err != nil {
		return err
	}
	if err := p.AwaitReady(ctx, target, timeout); err != nil {
		return err
	}
	sessions.RecordActivity(platformName, target)
	return nil
}
