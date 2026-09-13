package android

import (
	"testing"

	"github.com/espetro/mcp-sim/internal/config"
	"github.com/espetro/mcp-sim/pkg/contract"
	"github.com/espetro/mcp-sim/pkg/orchestrator/conformance"
)

// TestConformance runs the shared platform conformance suite against the
// real Android adapter. Skipped when the Android SDK (emulator/adb) is not
// installed — android.New returns (nil, nil) in that case.
func TestConformance(t *testing.T) {
	conformance.RunSuite(t, func(t *testing.T) (contract.Platform, func()) {
		ap, err := New(config.AndroidConfig{})
		if err != nil {
			t.Fatalf("android.New: %v", err)
		}
		if ap == nil {
			t.Skip("Android SDK not detected (emulator/adb not on PATH)")
		}
		return ap, func() {}
	})
}
