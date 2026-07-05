package contract

import (
	"context"
	"time"
)

// Platform abstracts emulator/simulator lifecycle operations.
type Platform interface {
	// Name returns the platform identifier (e.g. "ios", "android").
	Name() string
	// List returns all available devices (running or not).
	List(ctx context.Context) ([]Device, error)
	// Start boots a device and returns its updated state.
	Start(ctx context.Context, target string, opts StartOpts) (Device, error)
	// Stop shuts down a running device.
	Stop(ctx context.Context, target string) error
	// State returns the current state of a device.
	State(ctx context.Context, target string) (DeviceState, error)
	// AwaitReady blocks until the device is fully booted or the timeout fires.
	AwaitReady(ctx context.Context, target string, timeout time.Duration) error
	// Wipe erases the device's user data (boot fresh).
	Wipe(ctx context.Context, target string) error
	// OpenURL launches a deep link on the device.
	OpenURL(ctx context.Context, target, url string) error
}
