package contract

import (
	"context"
	"testing"
	"time"
)

func TestCapabilitySetHasEnable(t *testing.T) {
	var s CapabilitySet
	for _, c := range []Capability{CapList, CapStart, CapStop, CapState, CapAwaitReady, CapWipe, CapOpenURL, CapOptimize, CapMeasure} {
		if s.Has(c) {
			t.Fatalf("zero set should not have %v", c)
		}
		s = s.Enable(c)
		if !s.Has(c) {
			t.Fatalf("after Enable(%v), Has(%v) = false", c, c)
		}
	}
	// Enable is non-destructive: other bits survive.
	if !s.Has(CapList) || !s.Has(CapMeasure) {
		t.Fatal("Enable should preserve previously enabled bits")
	}
}

func TestCapabilitySetString(t *testing.T) {
	if got := CapabilitySet(0).String(); got != "none" {
		t.Fatalf("empty set String() = %q, want %q", got, "none")
	}
	s := CapAll.Enable(CapOptimize).Enable(CapMeasure)
	want := "list|start|stop|state|await_ready|wipe|open_url|optimize|measure"
	if got := s.String(); got != want {
		t.Fatalf("String() = %q, want %q", got, want)
	}
	// Round trip: String output only contains known caps.
	if !CapAll.Has(CapList) || CapAll.Has(CapOptimize) {
		t.Fatal("CapAll should be base caps without optimize/measure")
	}
}

type fakeOpt struct{ base }

type base struct{}

func TestCapabilitiesFor(t *testing.T) {
	plat := optPlatform{}
	s := CapabilitiesFor(plat)
	if !s.Has(CapOptimize) || !s.Has(CapMeasure) {
		t.Fatalf("CapabilitiesFor(Optimizer impl) missing optimize/measure: %v", s)
	}
	s2 := CapabilitiesFor(barePlatform{})
	if s2.Has(CapOptimize) || s2.Has(CapMeasure) {
		t.Fatalf("CapabilitiesFor(plain platform) should lack optimize/measure: %v", s2)
	}
}

type barePlatform struct{}

func (barePlatform) Name() string                               { return "" }
func (barePlatform) List(ctx context.Context) ([]Device, error) { return nil, nil }
func (barePlatform) Start(ctx context.Context, target string, o StartOpts) (Device, error) {
	return Device{}, nil
}
func (barePlatform) Stop(ctx context.Context, target string) error                        { return nil }
func (barePlatform) State(ctx context.Context, target string) (DeviceState, error)        { return "", nil }
func (barePlatform) AwaitReady(ctx context.Context, target string, d time.Duration) error { return nil }
func (barePlatform) Wipe(ctx context.Context, target string) error                        { return nil }
func (barePlatform) OpenURL(ctx context.Context, target, url string) error                { return nil }
func (barePlatform) Capabilities() CapabilitySet                                          { return CapAll }

type optPlatform struct{ barePlatform }

func (optPlatform) Optimize(ctx context.Context, target string, o OptimizeOpts) error { return nil }
func (optPlatform) Restore(ctx context.Context, target string) error                  { return nil }
func (optPlatform) OptimizeStatus(ctx context.Context, target string) (OptimizeStatus, error) {
	return OptimizeStatus{}, nil
}
func (optPlatform) Measure(ctx context.Context, target string) (ResourceUsage, error) {
	return ResourceUsage{}, nil
}

func TestAs(t *testing.T) {
	p := optPlatform{}
	if _, ok := As[Optimizer](p); !ok {
		t.Fatal("As[Optimizer] should succeed for optimizer platform")
	}
	if _, ok := As[Optimizer](barePlatform{}); ok {
		t.Fatal("As[Optimizer] should fail for plain platform")
	}
	if _, ok := As[barePlatform](p); !ok {
		t.Log("note: As[concrete] does not see through embedding of different type")
	}
}
