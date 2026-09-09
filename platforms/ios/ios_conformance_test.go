package ios

import (
	"context"
	"os/exec"
	"runtime"
	"testing"

	"github.com/espetro/mcp-sim/internal/config"
	"github.com/espetro/mcp-sim/pkg/contract"
	"github.com/espetro/mcp-sim/pkg/orchestrator/conformance"
)

// TestConformance runs the shared platform conformance suite against the
// real iOS adapter. Requires macOS with Xcode (xcrun simctl); skipped
// elsewhere or when no devices are listable.
func TestConformance(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("iOS simulator adapter requires macOS")
	}
	if err := exec.CommandContext(context.Background(), "xcrun", "simctl", "help").Run(); err != nil {
		t.Skipf("xcrun simctl unavailable: %v", err)
	}
	conformance.RunSuite(t, func(t *testing.T) (contract.Platform, func()) {
		p, err := New(context.Background(), config.IOSConfig{})
		if err != nil {
			t.Fatalf("ios.New: %v", err)
		}
		if p == nil {
			t.Skip("Xcode developer directory not detected")
		}
		devices, err := p.List(context.Background())
		if err != nil {
			t.Skipf("simctl list failed: %v", err)
		}
		if len(devices) == 0 {
			t.Skip("no iOS simulators installed")
		}
		return p, func() {}
	})
}
