package orchestrator

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/espetro/mcp-sim/pkg/contract"
)

// fakePlatform implements contract.Platform, plus optional Optimizer and
// profileReconciler, recording the adapter call sequence.
type fakePlatform struct {
	name     string
	dev      contract.Device
	devState contract.DeviceState
	caps     contract.CapabilitySet

	calls     []string
	stateErr  error
	listErr   error
	slimmed   bool
	withOptim bool // implement Optimizer + advertise CapOptimize
}

func (f *fakePlatform) Name() string { return f.name }

func (f *fakePlatform) Capabilities() contract.CapabilitySet {
	if f.caps != 0 {
		return f.caps
	}
	return contract.CapAll
}

func (f *fakePlatform) List(ctx context.Context) ([]contract.Device, error) {
	if f.listErr != nil {
		return nil, f.listErr
	}
	return []contract.Device{f.dev}, nil
}

func (f *fakePlatform) Start(ctx context.Context, target string, opts contract.StartOpts) (contract.Device, error) {
	f.calls = append(f.calls, "Start")
	f.devState = contract.DeviceStateRunning
	return f.dev, nil
}

func (f *fakePlatform) Stop(ctx context.Context, target string) error {
	f.calls = append(f.calls, "Stop")
	f.devState = contract.DeviceStateStopped
	return nil
}

func (f *fakePlatform) State(ctx context.Context, target string) (contract.DeviceState, error) {
	if f.stateErr != nil {
		return contract.DeviceStateUnknown, f.stateErr
	}
	return f.devState, nil
}

func (f *fakePlatform) AwaitReady(ctx context.Context, target string, timeout time.Duration) error {
	f.calls = append(f.calls, "AwaitReady")
	return nil
}

func (f *fakePlatform) Wipe(ctx context.Context, target string) error {
	f.calls = append(f.calls, "Wipe")
	f.slimmed = false
	return nil
}

func (f *fakePlatform) OpenURL(ctx context.Context, target, url string) error {
	f.calls = append(f.calls, "OpenURL")
	return nil
}

func (f *fakePlatform) Optimize(ctx context.Context, target string, o contract.OptimizeOpts) error {
	f.calls = append(f.calls, "Optimize")
	f.slimmed = true
	return nil
}

func (f *fakePlatform) Restore(ctx context.Context, target string) error {
	f.calls = append(f.calls, "Restore")
	f.slimmed = false
	return nil
}

func (f *fakePlatform) OptimizeStatus(ctx context.Context, target string) (contract.OptimizeStatus, error) {
	return contract.OptimizeStatus{Slimmed: f.slimmed}, nil
}

func (f *fakePlatform) Measure(ctx context.Context, target string) (contract.ResourceUsage, error) {
	return contract.ResourceUsage{PhysFootprintBytes: 42}, nil
}

// ReconcileProfile: idempotent by contract — only optimizes when not slimmed.
func (f *fakePlatform) ReconcileProfile(ctx context.Context, target string) error {
	f.calls = append(f.calls, "ReconcileProfile")
	if !f.slimmed {
		if err := f.Optimize(ctx, target, contract.OptimizeOpts{}); err != nil {
			return err
		}
	}
	return nil
}

func newFake(name string, state contract.DeviceState) *fakePlatform {
	return &fakePlatform{
		name:     name,
		devState: state,
		dev:      contract.Device{ID: name + "-dev", Name: name, Platform: name, State: state},
	}
}

func withOptimizer(f *fakePlatform) *fakePlatform {
	f.withOptim = true
	return f
}

func TestRegisterDuplicatePlatform(t *testing.T) {
	o, err := New()
	if err != nil {
		t.Fatal(err)
	}
	if err := o.RegisterPlatform(newFake("ios", contract.DeviceStateStopped)); err != nil {
		t.Fatal(err)
	}
	if err := o.RegisterPlatform(newFake("ios", contract.DeviceStateStopped)); err == nil {
		t.Fatal("want duplicate registration error")
	}
}

func TestBootRunningIsIdempotent(t *testing.T) {
	f := withOptimizer(newFake("ios", contract.DeviceStateRunning))
	o, err := New(WithPlatform(f))
	if err != nil {
		t.Fatal(err)
	}
	dev, err := o.Boot(context.Background(), "ios", "ios-dev", contract.StartOpts{})
	if err != nil {
		t.Fatalf("boot on running device must succeed, got %v", err)
	}
	if dev.State != contract.DeviceStateRunning {
		t.Fatalf("want Running device, got %s", dev.State)
	}
	for _, c := range f.calls {
		if c == "Start" {
			t.Fatal("Start must not be called for already-running device")
		}
	}
	// ReconcileProfile called exactly once (the fake's own Optimize may follow).
	n := 0
	for _, c := range f.calls {
		if c == "ReconcileProfile" {
			n++
		}
	}
	if n != 1 {
		t.Fatalf("want ReconcileProfile called once, got %d (calls: %v)", n, f.calls)
	}
}

func TestBootFromStopped(t *testing.T) {
	f := newFake("ios", contract.DeviceStateStopped)
	o, _ := New(WithPlatform(f))
	if _, err := o.Boot(context.Background(), "ios", "ios-dev", contract.StartOpts{}); err != nil {
		t.Fatal(err)
	}
	want := []string{"Start", "AwaitReady", "ReconcileProfile", "Optimize"}
	if !eq(f.calls, want) {
		t.Fatalf("want %v, got %v", want, f.calls)
	}
}

func TestStopStoppedIsNoop(t *testing.T) {
	f := newFake("ios", contract.DeviceStateStopped)
	o, _ := New(WithPlatform(f))
	if err := o.Stop(context.Background(), "ios", "ios-dev"); err != nil {
		t.Fatalf("stopping a stopped device must be nil, got %v", err)
	}
	if len(f.calls) != 0 {
		t.Fatalf("adapter must not be touched, got %v", f.calls)
	}
}

func TestWipeRunningSequence(t *testing.T) {
	f := newFake("ios", contract.DeviceStateRunning)
	o, _ := New(WithPlatform(f))
	if err := o.Wipe(context.Background(), "ios", "ios-dev"); err != nil {
		t.Fatal(err)
	}
	want := []string{"Stop", "Wipe", "ReconcileProfile", "Optimize"}
	if !eq(f.calls, want) {
		t.Fatalf("want %v, got %v", want, f.calls)
	}
}

func TestWipeStoppedSequence(t *testing.T) {
	f := newFake("ios", contract.DeviceStateStopped)
	o, _ := New(WithPlatform(f))
	if err := o.Wipe(context.Background(), "ios", "ios-dev"); err != nil {
		t.Fatal(err)
	}
	want := []string{"Wipe", "ReconcileProfile", "Optimize"}
	if !eq(f.calls, want) {
		t.Fatalf("want %v, got %v", want, f.calls)
	}
}

func TestWipePolicyOff(t *testing.T) {
	f := newFake("ios", contract.DeviceStateStopped)
	o, _ := New(WithPlatform(f), WithWipePolicy(false))
	if err := o.Wipe(context.Background(), "ios", "ios-dev"); err != nil {
		t.Fatal(err)
	}
	want := []string{"Wipe"}
	if !eq(f.calls, want) {
		t.Fatalf("want %v, got %v", want, f.calls)
	}
}

func TestListPartialFailure(t *testing.T) {
	ok := newFake("ios", contract.DeviceStateStopped)
	bad := newFake("android", contract.DeviceStateStopped)
	bad.listErr = errors.New("boom")
	o, _ := New(WithPlatform(ok), WithPlatform(bad))
	devs, err := o.List(context.Background())
	if err != nil {
		t.Fatalf("partial failure must not error, got %v", err)
	}
	if len(devs) != 1 || devs[0].ID != "ios-dev" {
		t.Fatalf("want only ios device, got %v", devs)
	}
}

func TestListAllFail(t *testing.T) {
	a := newFake("ios", contract.DeviceStateStopped)
	a.listErr = errors.New("boom")
	b := newFake("android", contract.DeviceStateStopped)
	b.listErr = errors.New("boom")
	o, _ := New(WithPlatform(a), WithPlatform(b))
	if _, err := o.List(context.Background()); err == nil {
		t.Fatal("total failure must error")
	}
}

func TestListEmptyRegistry(t *testing.T) {
	o, _ := New()
	devs, err := o.List(context.Background())
	if err != nil || len(devs) != 0 {
		t.Fatalf("want empty nil-error, got %v %v", devs, err)
	}
}

func TestOptimizerStateNoCapability(t *testing.T) {
	f := newFake("ios", contract.DeviceStateRunning)
	f.caps = contract.CapAll // no CapOptimize
	o, _ := New(WithPlatform(f))
	st, usage, err := o.OptimizerState(context.Background(), "ios", "ios-dev")
	if err != nil || st != nil || usage != nil {
		t.Fatalf("want (nil,nil,nil), got %v %v %v", st, usage, err)
	}
}

func TestOptimizerStateWithCapability(t *testing.T) {
	f := newFake("ios", contract.DeviceStateRunning)
	f.caps = contract.CapAll.Enable(contract.CapOptimize).Enable(contract.CapMeasure)
	o, _ := New(WithPlatform(f))
	st, usage, err := o.OptimizerState(context.Background(), "ios", "ios-dev")
	if err != nil || st == nil || usage == nil {
		t.Fatalf("want status+usage, got %v %v %v", st, usage, err)
	}
}

func TestOptionsNilAndDuplicate(t *testing.T) {
	if _, err := New(WithPlatform(nil)); err == nil {
		t.Fatal("nil platform must error")
	}
	f := newFake("ios", contract.DeviceStateStopped)
	if _, err := New(WithPlatform(f), WithPlatform(newFake("ios", contract.DeviceStateStopped))); err == nil {
		t.Fatal("duplicate WithPlatform must error")
	}
	if _, err := New(WithController(nil)); err == nil {
		t.Fatal("nil controller must error")
	}
}

func eq(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := 0; i < len(got) && i < len(want); i++ {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}
