package contract

import (
	"context"

	"errors"
)

// Verifier errors.
var (
	// ErrVerifierMissing is returned when no verifier backend is
	// configured or its controller binary is missing.
	ErrVerifierMissing = errors.New("verifier: no verifier backend available")
)

// SnapshotNode is a single accessibility element in a Snapshot. Ref is stable
// within one snapshot; pass it back to Verifier.Tap/Type.
type SnapshotNode struct {
	Ref     string `json:"ref"`             // Stable within one snapshot
	Role    string `json:"role"`            // e.g. "Button", "TextField", "Cell"
	Label   string `json:"label,omitempty"` // Accessibility label / text
	Value   string `json:"value,omitempty"` // Current value (inputs)
	X       int    `json:"x"`               // Frame origin, points
	Y       int    `json:"y"`               // Frame origin, points
	Width   int    `json:"width"`           // Frame size, points
	Height  int    `json:"height"`          // Frame size, points
	Enabled bool   `json:"enabled"`
}

// Snapshot is an LLM-friendly view of the current screen.
type Snapshot struct {
	Platform string         `json:"platform"`
	Target   string         `json:"target"`
	Nodes    []SnapshotNode `json:"nodes"`
}

// Screenshot is a captured device screen.
type Screenshot struct {
	Platform string `json:"platform"`
	Target   string `json:"target"`
	Format   string `json:"format"` // e.g. "png"
	Base64   string `json:"base64"` // Image bytes, base64 encoded
}

// Verifier is the contract for interacting with a booted device: taps,
// gestures, text entry and screen reads. Implementations drive a controller
// backend (agent-device today; others later) selected at startup. A nil or
// unconfigured Verifier degrades gracefully: orchestrator methods return
// verifier_unavailable ToolErrors.
type Verifier interface {
	// Name returns the backend identifier (e.g. "agent-device").
	Name() string
	// Ping checks that the backend is reachable.
	Ping(ctx context.Context) error
	// Tap taps the element with the given snapshot ref.
	Tap(ctx context.Context, target, ref string) error
	// DoubleTap double-taps the element with the given snapshot ref.
	DoubleTap(ctx context.Context, target, ref string) error
	// LongPress long-presses the element with the given snapshot ref.
	LongPress(ctx context.Context, target, ref string, ms int) error
	// Swipe swipes in a direction ("up", "down", "left", "right").
	Swipe(ctx context.Context, target, direction string) error
	// Type enters text into the element with the given snapshot ref.
	Type(ctx context.Context, target, ref, text string) error
	// PressButton presses a hardware/system button (e.g. "home", "back").
	PressButton(ctx context.Context, target, button string) error
	// OpenURL opens a URL or deep link on the device.
	OpenURL(ctx context.Context, target, url string) error
	// Snapshot captures the current screen hierarchy.
	Snapshot(ctx context.Context, target string) (Snapshot, error)
	// Screenshot captures the current screen as an image.
	Screenshot(ctx context.Context, target string) (Screenshot, error)
}
