package core

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/espetro/mcp-sim/pkg/contract"
)

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func TestReserveAndOwner(t *testing.T) {
	m := NewManager(time.Hour, discardLogger())
	ctx := context.Background()

	if err := m.Reserve(ctx, "ios", "sim1", "session-a"); err != nil {
		t.Fatalf("Reserve failed: %v", err)
	}

	owner, ok := m.Owner("ios", "sim1")
	if !ok || owner != "session-a" {
		t.Fatalf("Owner = (%q, %v), want (session-a, true)", owner, ok)
	}

	if err := m.CheckAccess("ios", "sim1", "session-a"); err != nil {
		t.Fatalf("CheckAccess by owner failed: %v", err)
	}

	var te *ToolError
	if err := m.CheckAccess("ios", "sim1", "session-b"); !errors.As(err, &te) || te.Code != contract.ErrDeviceReserved {
		t.Fatalf("CheckAccess by other session returned %v, want device_reserved", err)
	}
}

func TestReserveIdempotent(t *testing.T) {
	m := NewManager(time.Hour, discardLogger())
	ctx := context.Background()

	if err := m.Reserve(ctx, "ios", "sim1", "session-a"); err != nil {
		t.Fatalf("first Reserve failed: %v", err)
	}
	if err := m.Reserve(ctx, "ios", "sim1", "session-a"); err != nil {
		t.Fatalf("second Reserve by same session failed: %v", err)
	}
	if err := m.Reserve(ctx, "ios", "sim1", "session-b"); err == nil {
		t.Fatal("Reserve by different session should fail")
	}
}

func TestRelease(t *testing.T) {
	m := NewManager(time.Hour, discardLogger())
	ctx := context.Background()

	if err := m.Reserve(ctx, "ios", "sim1", "session-a"); err != nil {
		t.Fatalf("Reserve failed: %v", err)
	}
	m.Release("ios", "sim1")

	if _, ok := m.Owner("ios", "sim1"); ok {
		t.Fatal("Owner should be false after Release")
	}
	if err := m.CheckAccess("ios", "sim1", "session-b"); err != nil {
		t.Fatalf("CheckAccess after release failed: %v", err)
	}
}

func TestCheckAccessUnreserved(t *testing.T) {
	m := NewManager(time.Hour, discardLogger())
	if err := m.CheckAccess("ios", "sim1", "any-session"); err != nil {
		t.Fatalf("CheckAccess on unreserved device failed: %v", err)
	}
}

func TestRecordActivityOnlyWhenReserved(t *testing.T) {
	m := NewManager(time.Hour, discardLogger())
	ctx := context.Background()

	base := time.Date(2026, 7, 5, 0, 0, 0, 0, time.UTC)
	m.now = func() time.Time { return base }
	m.RecordActivity("ios", "sim1")

	if err := m.Reserve(ctx, "ios", "sim1", "session-a"); err != nil {
		t.Fatalf("Reserve failed: %v", err)
	}
	m.now = func() time.Time { return base.Add(2 * time.Hour) }
	m.RecordActivity("ios", "sim1")
}

func TestSweeperStopsIdleAndReleases(t *testing.T) {
	m := NewManager(time.Hour, discardLogger())
	ctx := context.Background()

	stopped := make(map[sessionKey]bool)
	m.SetStopper(func(_ context.Context, platform, target, owner string) error {
		stopped[sessionKey{platform, target}] = true
		if owner != "session-a" {
			return errors.New("unexpected owner")
		}
		return nil
	})

	base := time.Date(2026, 7, 5, 0, 0, 0, 0, time.UTC)
	m.now = func() time.Time { return base }

	if err := m.Reserve(ctx, "ios", "sim1", "session-a"); err != nil {
		t.Fatalf("Reserve failed: %v", err)
	}

	m.now = func() time.Time { return base.Add(2 * time.Hour) }
	m.sweep(ctx)

	if !stopped[sessionKey{"ios", "sim1"}] {
		t.Fatal("sweeper did not stop idle device")
	}
	if _, ok := m.Owner("ios", "sim1"); ok {
		t.Fatal("reservation should be released after idle sweep")
	}
}

func TestSweeperDoesNotStopActive(t *testing.T) {
	m := NewManager(time.Hour, discardLogger())
	ctx := context.Background()

	stopped := false
	m.SetStopper(func(_ context.Context, _, _, _ string) error {
		stopped = true
		return nil
	})

	base := time.Date(2026, 7, 5, 0, 0, 0, 0, time.UTC)
	m.now = func() time.Time { return base }

	if err := m.Reserve(ctx, "ios", "sim1", "session-a"); err != nil {
		t.Fatalf("Reserve failed: %v", err)
	}

	m.now = func() time.Time { return base.Add(30 * time.Minute) }
	m.sweep(ctx)

	if stopped {
		t.Fatal("sweeper stopped active device")
	}
	if _, ok := m.Owner("ios", "sim1"); !ok {
		t.Fatal("active reservation should remain")
	}
}

func TestSweeperDoesNotReleaseChangedOwner(t *testing.T) {
	m := NewManager(time.Hour, discardLogger())
	ctx := context.Background()

	stopped := make(map[sessionKey]bool)
	m.SetStopper(func(_ context.Context, platform, target, owner string) error {
		stopped[sessionKey{platform, target}] = true
		// Simulate another goroutine claiming the device while stopper runs.
		key := sessionKey{platform, target}
		m.mu.Lock()
		m.reservations[key] = "session-c"
		m.mu.Unlock()
		return nil
	})

	base := time.Date(2026, 7, 5, 0, 0, 0, 0, time.UTC)
	m.now = func() time.Time { return base }

	if err := m.Reserve(ctx, "ios", "sim1", "session-a"); err != nil {
		t.Fatalf("Reserve failed: %v", err)
	}

	m.now = func() time.Time { return base.Add(2 * time.Hour) }
	m.sweep(ctx)

	if !stopped[sessionKey{"ios", "sim1"}] {
		t.Fatal("sweeper should still attempt to stop the idle key")
	}
	owner, ok := m.Owner("ios", "sim1")
	if !ok || owner != "session-c" {
		t.Fatalf("Owner = (%q, %v), want (session-c, true)", owner, ok)
	}
}

func TestStartReturnsImmediatelyWhenDisabled(t *testing.T) {
	m := NewManager(0, discardLogger())
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	done := make(chan struct{})
	go func() {
		m.Start(ctx)
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Start did not return immediately with timeout=0")
	}
}
