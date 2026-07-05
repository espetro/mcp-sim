package contract

import (
	"time"
)

// DeviceState represents the state of a device.
type DeviceState string

const (
	DeviceStateUnknown DeviceState = "unknown"
	DeviceStateStopped DeviceState = "stopped"
	DeviceStateBooting DeviceState = "booting"
	DeviceStateRunning DeviceState = "running"
	DeviceStateError   DeviceState = "error"
)

// Device represents a mobile emulator or simulator.
type Device struct {
	ID       string      `json:"id"`       // UDID for iOS, serial for Android
	Name     string      `json:"name"`     // Human-readable name
	Platform string      `json:"platform"` // "ios" or "android"
	State    DeviceState `json:"state"`
	// OS version if known
	Version string `json:"version,omitempty"`
}

// StartOpts controls how a device is started.
type StartOpts struct {
	NoWindow bool          `json:"no_window,omitempty"` // Headless mode
	Port     int           `json:"port,omitempty"`      // Explicit port (Android)
	Timeout  time.Duration `json:"timeout,omitempty"`   // Boot timeout
}

// ProxyInfo holds the network address of a running controller proxy.
type ProxyInfo struct {
	Name    string `json:"name"`
	URL     string `json:"url"` // e.g. http://127.0.0.1:9000
	Running bool   `json:"running"`
}

// StartConfig configures a controller being started.
type StartConfig struct {
	Port int `json:"port"` // Proxy port
}

// Tool errors (stable, parseable codes returned via MCP ToolResultError).
const (
	ErrDeviceNotFound        = "device_not_found"
	ErrAlreadyRunning        = "already_running"
	ErrNotRunning            = "not_running"
	ErrTimeout               = "timeout"
	ErrUnsupportedPlatform   = "unsupported_platform"
	ErrUnsupportedController = "unsupported_controller"
	ErrInternal              = "internal"
)
