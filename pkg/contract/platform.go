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
	// Capabilities reports the bitmask of supported operations. Required
	// (pre-1.0 contract): advertise CapOptimize/CapMeasure only when the
	// platform also implements Optimizer. See capabilities.go.
	Capabilities() CapabilitySet
}

// Optimizer is an optional Platform extension that reduces a device's
// resource usage (e.g. disabling daemons an iOS simulator does not need).
// Platforms implement it only when an optimizer backend is configured;
// the registry type-asserts Platform.(Optimizer) to wire the behavior.
// The MCP tool surface stays unchanged: optimization folds into boot
// options and state output.
type Optimizer interface {
	// Optimize slims the device (e.g. `simslim on <target>`).
	Optimize(ctx context.Context, target string, o OptimizeOpts) error
	// Restore returns the device to stock (e.g. `simslim off <target>`).
	Restore(ctx context.Context, target string) error
	// OptimizeStatus reports whether the device is currently optimized.
	OptimizeStatus(ctx context.Context, target string) (OptimizeStatus, error)
	// Measure returns the device's current resource usage.
	Measure(ctx context.Context, target string) (ResourceUsage, error)
}

// AppInstaller is an optional Platform extension that installs an app
// artifact onto a device. iOS: `simctl install`; Android: `adb install -r`.
type AppInstaller interface {
	// InstallApp installs an app artifact (bundle path or APK) on the device.
	InstallApp(ctx context.Context, target, artifactPath string) error
}

// AppLauncher is an optional Platform extension that launches an installed
// app by bundle identifier. iOS: `simctl launch` (returns pid); Android:
// `am start` after resolving the launch activity.
type AppLauncher interface {
	// LaunchApp starts the app and returns its process id (0 when the
	// platform cannot report one).
	LaunchApp(ctx context.Context, target, bundleID string) (pid int, err error)
}

// OptimizeOpts controls how a device is optimized.
type OptimizeOpts struct {
	Profile  string   `json:"profile,omitempty"`   // Path to a profile JSON file; empty = default slim profile
	Except   []string `json:"except,omitempty"`    // Category IDs to leave enabled
	Keep     []string `json:"keep,omitempty"`      // Individual daemon labels to keep running
	NoReboot bool     `json:"no_reboot,omitempty"` // Slim current boot session only (required on iOS < 18.5)
}

// OptimizeStatus reports how optimized a device is.
type OptimizeStatus struct {
	Slimmed         bool `json:"slimmed"`          // Any managed daemons disabled
	Persistent      bool `json:"persistent"`       // Runtime keeps disabled state across reboot
	ManagedDisabled int  `json:"managed_disabled"` // Managed labels currently disabled
	ManagedTotal    int  `json:"managed_total"`    // Size of the managed universe
}

// ResourceUsage is a device resource snapshot (phys_footprint based).
type ResourceUsage struct {
	PhysFootprintBytes int64   `json:"phys_footprint_bytes"` // Summed phys_footprint (compressed + dirty)
	ProcessCount       int     `json:"process_count"`
	CPUPercent         float64 `json:"cpu_percent,omitempty"`
}
