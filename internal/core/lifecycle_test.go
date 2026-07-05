package core

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/espetro/mcp-sim/pkg/contract"
)

type fakePlatform struct {
	mu      sync.Mutex
	name    string
	state   contract.DeviceState
	started bool
	stopped bool
}

func (p *fakePlatform) Name() string { return p.name }

func (p *fakePlatform) List(ctx context.Context) ([]contract.Device, error) {
	return []contract.Device{{ID: "dev1", Name: "dev1", Platform: p.name, State: p.state}}, nil
}

func (p *fakePlatform) Start(ctx context.Context, target string, opts contract.StartOpts) (contract.Device, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.state = contract.DeviceStateRunning
	p.started = true
	return contract.Device{ID: target, Name: target, Platform: p.name, State: p.state}, nil
}

func (p *fakePlatform) Stop(ctx context.Context, target string) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.state = contract.DeviceStateStopped
	p.stopped = true
	return nil
}

func (p *fakePlatform) State(ctx context.Context, target string) (contract.DeviceState, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.state, nil
}

func (p *fakePlatform) AwaitReady(ctx context.Context, target string, timeout time.Duration) error {
	return nil
}

func (p *fakePlatform) Wipe(ctx context.Context, target string) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.state = contract.DeviceStateStopped
	return nil
}

func (p *fakePlatform) OpenURL(ctx context.Context, target, url string) error {
	return nil
}

func newTestLifecycle(p contract.Platform) (*Lifecycle, *Manager) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	reg := NewRegistry(logger)
	reg.RegisterPlatform(p)
	sessions := NewManager(time.Hour, logger)
	return NewLifecycle(reg, sessions), sessions
}

func TestBootDeviceReservesAndSecondSessionDenied(t *testing.T) {
	fp := &fakePlatform{name: "ios", state: contract.DeviceStateStopped}
	lc, sessions := newTestLifecycle(fp)
	ctx := context.Background()

	if _, err := lc.BootDevice(ctx, "ios", "dev1", "session-a", contract.StartOpts{}); err != nil {
		t.Fatalf("BootDevice failed: %v", err)
	}

	owner, ok := sessions.Owner("ios", "dev1")
	if !ok || owner != "session-a" {
		t.Fatalf("Owner = (%q, %v), want (session-a, true)", owner, ok)
	}

	_, err := lc.BootDevice(ctx, "ios", "dev1", "session-b", contract.StartOpts{})
	var te *ToolError
	if !errors.As(err, &te) || te.Code != contract.ErrDeviceReserved {
		t.Fatalf("BootDevice by other session returned %v, want device_reserved", err)
	}
}

func TestBootDeviceIdempotentSameSession(t *testing.T) {
	fp := &fakePlatform{name: "ios", state: contract.DeviceStateStopped}
	lc, _ := newTestLifecycle(fp)
	ctx := context.Background()

	if _, err := lc.BootDevice(ctx, "ios", "dev1", "session-a", contract.StartOpts{}); err != nil {
		t.Fatalf("first BootDevice failed: %v", err)
	}
	// Second boot by the same session is allowed through the reservation gate.
	// Because the fake platform reports running, it returns already_running.
	_, err := lc.BootDevice(ctx, "ios", "dev1", "session-a", contract.StartOpts{})
	var te *ToolError
	if !errors.As(err, &te) || te.Code != contract.ErrAlreadyRunning {
		t.Fatalf("second BootDevice returned %v, want already_running", err)
	}
}

func TestStopDeviceReleasesReservation(t *testing.T) {
	fp := &fakePlatform{name: "ios", state: contract.DeviceStateRunning}
	lc, sessions := newTestLifecycle(fp)
	ctx := context.Background()

	if err := lc.StopDevice(ctx, "ios", "dev1", "session-a"); err != nil {
		t.Fatalf("StopDevice failed: %v", err)
	}

	if _, ok := sessions.Owner("ios", "dev1"); ok {
		t.Fatal("reservation should be released after StopDevice")
	}

	// After release, another session can claim the device.
	if _, err := lc.BootDevice(ctx, "ios", "dev1", "session-b", contract.StartOpts{}); err != nil {
		t.Fatalf("BootDevice by another session after stop failed: %v", err)
	}
}

func TestStopDeviceDeniedForOtherSession(t *testing.T) {
	fp := &fakePlatform{name: "ios", state: contract.DeviceStateStopped}
	lc, _ := newTestLifecycle(fp)
	ctx := context.Background()

	// Boot reserves the device to session-a.
	if _, err := lc.BootDevice(ctx, "ios", "dev1", "session-a", contract.StartOpts{}); err != nil {
		t.Fatalf("BootDevice failed: %v", err)
	}

	if err := lc.StopDevice(ctx, "ios", "dev1", "session-b"); err == nil {
		t.Fatal("StopDevice by other session should fail")
	}
}
