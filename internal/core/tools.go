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

// GetOptimizerState returns the optimizer status (and live memory usage when
// the device is booted) for a device, or (nil, nil, nil) when the platform
// has no optimizer. Used by get_state to fold optimizer info into its output.
func GetOptimizerState(ctx context.Context, registry *Registry, platformName, target string) (*contract.OptimizeStatus, *contract.ResourceUsage, error) {
	p, ok := registry.PlatformByName(platformName)
	if !ok {
		return nil, nil, &ToolError{Code: contract.ErrUnsupportedPlatform, Msg: "platform not found: " + platformName}
	}
	opt, ok := p.(contract.Optimizer)
	if !ok {
		return nil, nil, nil
	}
	st, err := opt.OptimizeStatus(ctx, target)
	if err != nil {
		return nil, nil, err
	}
	var usage *contract.ResourceUsage
	// Measure only works on a booted device; ignore errors (e.g. device just
	// shut down) rather than failing get_state.
	if state, err := p.State(ctx, target); err == nil && state == contract.DeviceStateRunning {
		if m, err := opt.Measure(ctx, target); err == nil {
			usage = &m
		}
	}
	return &st, usage, nil
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
