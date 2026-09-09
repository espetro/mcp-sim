// Package conformance provides an interchangeability test suite for
// contract.Platform implementations, exercised through pkg/orchestrator.
//
// Every adapter (in-tree or third-party) should run RunSuite in its own
// test package:
//
//	func TestConformance(t *testing.T) {
//	    conformance.RunSuite(t, func(t *testing.T) (contract.Platform, func()) {
//	        p, err := myadapter.New(cfg)
//	        if err != nil { t.Fatal(err) }
//	        if p == nil { t.Skip("SDK not installed") }
//	        return p, func() {}
//	    })
//	}
//
// Factories for real-device adapters should t.Skip when no SDK or device is
// available; the Factory decides how much of the suite can run. Fakes can
// run the full suite on every platform in CI.
package conformance

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/espetro/mcp-sim/pkg/contract"
	"github.com/espetro/mcp-sim/pkg/orchestrator"
)

// Factory returns a fresh Platform under test plus a cleanup func. The
// cleanup runs after the suite finishes (or skips) and must be safe to call
// even when the platform was never exercised.
type Factory func(t *testing.T) (contract.Platform, func())

// awaitTimeout is deliberately short: the suite must not stall CI. Real
// adapters with a cold device may exceed it, which the suite tolerates as a
// non-unsupported error.
const awaitTimeout = 30 * time.Second

// RunSuite exercises p through a fresh orchestrator: list, state of a known
// target, stop idempotency, and capability honesty (every advertised
// capability's operation must work, i.e. never return ErrUnsupported*).
// All checks run as subtests so failures are granular.
func RunSuite(t *testing.T, f Factory) {
	t.Helper()

	p, cleanup := f(t)
	if cleanup != nil {
		defer cleanup()
	}
	if p == nil {
		t.Skip("platform factory returned nil (SDK/device unavailable)")
	}

	o, err := orchestrator.New(orchestrator.WithPlatform(p))
	if err != nil {
		t.Fatalf("orchestrator.New: %v", err)
	}
	name := p.Name()
	caps := p.Capabilities()

	target := firstDevice(t, o, name)

	t.Run("list", func(t *testing.T) {
		devs, err := o.List(context.Background())
		if err != nil {
			t.Fatalf("List: %v", err)
		}
		for _, d := range devs {
			if d.ID == "" {
				t.Errorf("device with empty ID: %+v", d)
			}
		}
	})

	t.Run("state_known_target", func(t *testing.T) {
		if target == "" {
			t.Skip("no device available to query")
		}
		st, err := o.State(context.Background(), name, target)
		if err != nil {
			t.Fatalf("State(%s): %v", target, err)
		}
		if st == contract.DeviceStateUnknown && err == nil {
			// Unknown with nil error is tolerated by contract semantics but
			// worth flagging for real adapters.
			t.Logf("State(%s) = unknown (tolerated)", target)
		}
	})

	t.Run("stop_idempotent", func(t *testing.T) {
		if target == "" {
			t.Skip("no device available to stop")
		}
		ctx := context.Background()
		if err := o.Stop(ctx, name, target); err != nil {
			t.Fatalf("first Stop: %v", err)
		}
		if err := o.Stop(ctx, name, target); err != nil {
			t.Fatalf("second Stop (stopping a stopped device must be a no-op success): %v", err)
		}
	})

	t.Run("capability_honesty", func(t *testing.T) {
		t.Run("CapList", func(t *testing.T) {
			if !caps.Has(contract.CapList) {
				t.Skip("CapList not advertised")
			}
			if _, err := o.List(context.Background()); isUnsupported(err) {
				t.Fatalf("CapList advertised but List returned unsupported: %v", err)
			}
		})
		t.Run("CapState", func(t *testing.T) {
			if !caps.Has(contract.CapState) {
				t.Skip("CapState not advertised")
			}
			if target == "" {
				t.Skip("no device available")
			}
			if _, err := o.State(context.Background(), name, target); isUnsupported(err) {
				t.Fatalf("CapState advertised but State returned unsupported: %v", err)
			}
		})
		t.Run("CapStop", func(t *testing.T) {
			if !caps.Has(contract.CapStop) {
				t.Skip("CapStop not advertised")
			}
			if target == "" {
				t.Skip("no device available")
			}
			if err := o.Stop(context.Background(), name, target); isUnsupported(err) {
				t.Fatalf("CapStop advertised but Stop returned unsupported: %v", err)
			}
		})
		t.Run("CapAwaitReady", func(t *testing.T) {
			if !caps.Has(contract.CapAwaitReady) {
				t.Skip("CapAwaitReady not advertised")
			}
			if target == "" {
				t.Skip("no device available")
			}
			// Short timeout acceptable: tolerate timeout and device-not-found
			// style errors; ErrUnsupported* must never appear.
			err := o.AwaitReady(context.Background(), name, target, awaitTimeout)
			if isUnsupported(err) {
				t.Fatalf("CapAwaitReady advertised but AwaitReady returned unsupported: %v", err)
			}
		})
		t.Run("CapWipe", func(t *testing.T) {
			if !caps.Has(contract.CapWipe) {
				t.Skip("CapWipe not advertised")
			}
			if target == "" {
				t.Skip("no device available")
			}
			// Wiping a real user's device is destructive; adapters under a
			// guarded real-device factory should decide via the Factory
			// whether to expose a wipe-safe target. We still call it so the
			// capability honesty check is meaningful, but tolerate
			// device-not-found style errors.
			if err := o.Wipe(context.Background(), name, target); isUnsupported(err) {
				t.Fatalf("CapWipe advertised but Wipe returned unsupported: %v", err)
			}
		})
		t.Run("CapOptimize", func(t *testing.T) {
			if !caps.Has(contract.CapOptimize) {
				t.Skip("CapOptimize not advertised")
			}
			if target == "" {
				t.Skip("no device available")
			}
			// OptimizerState must not error with unsupported when CapOptimize
			// is advertised. Device-not-found style errors are tolerated.
			if _, _, err := o.OptimizerState(context.Background(), name, target); isUnsupported(err) {
				t.Fatalf("CapOptimize advertised but OptimizerState returned unsupported: %v", err)
			}
		})
	})
}

// firstDevice lists through the orchestrator and returns the first device ID
// on the platform under test, or "" when none exist.
func firstDevice(t *testing.T, o *orchestrator.Orchestrator, platform string) string {
	t.Helper()
	devs, err := o.List(context.Background())
	if err != nil {
		t.Logf("List during target discovery failed (suite will skip target-dependent subtests): %v", err)
		return ""
	}
	for _, d := range devs {
		if d.Platform == platform {
			return d.ID
		}
	}
	return ""
}

// isUnsupported reports whether err carries an ErrUnsupported* code (via
// orchestrator.ToolError) or mentions "unsupported" — the one failure mode
// the suite never tolerates for an advertised capability.
func isUnsupported(err error) bool {
	if err == nil {
		return false
	}
	var te *orchestrator.ToolError
	if errors.As(err, &te) {
		return te.Code == contract.ErrUnsupportedPlatform || te.Code == contract.ErrUnsupportedController
	}
	return strings.Contains(err.Error(), "unsupported")
}
