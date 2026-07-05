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
func ListDevices(ctx context.Context, registry *Registry) ([]contract.Device, error) {
	var result []contract.Device
	for name, p := range registry.AllPlatforms() {
		devs, err := p.List(ctx)
		if err != nil {
			return nil, fmt.Errorf("listing devices on %s: %w", name, err)
		}
		result = append(result, devs...)
	}
	return result, nil
}

// GetDeviceState returns the state of a specific device.
func GetDeviceState(ctx context.Context, registry *Registry, platformName, target string) (contract.DeviceState, error) {
	p, ok := registry.PlatformByName(platformName)
	if !ok {
		return contract.DeviceStateUnknown, &ToolError{Code: contract.ErrUnsupportedPlatform, Msg: "platform not found: " + platformName}
	}
	state, err := p.State(ctx, target)
	if err != nil {
		return contract.DeviceStateUnknown, err
	}
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
func AwaitDeviceReady(ctx context.Context, registry *Registry, platformName, target string, timeout time.Duration) error {
	p, ok := registry.PlatformByName(platformName)
	if !ok {
		return &ToolError{Code: contract.ErrUnsupportedPlatform, Msg: "platform not found: " + platformName}
	}
	return p.AwaitReady(ctx, target, timeout)
}
