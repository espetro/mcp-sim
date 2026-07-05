package core

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/espetro/mcp-sim/pkg/contract"
)

// sessionKey identifies a specific device reservation.
type sessionKey struct {
	platform string
	target   string
}

// Manager tracks per-session device reservations and idle timeouts.
type Manager struct {
	mu           sync.Mutex
	reservations map[sessionKey]string
	activity     map[sessionKey]time.Time
	timeout      time.Duration
	logger       *slog.Logger
	stopper      func(context.Context, string, string, string) error
	now          func() time.Time
}

// NewManager creates a reservation manager with the given idle timeout.
// A timeout of 0 disables the idle sweeper.
func NewManager(timeout time.Duration, logger *slog.Logger) *Manager {
	return &Manager{
		reservations: make(map[sessionKey]string),
		activity:     make(map[sessionKey]time.Time),
		timeout:      timeout,
		logger:       logger,
		now:          time.Now,
	}
}

// SetStopper injects the callback used by the idle sweeper to stop a device.
// The callback receives the owning session so it can pass access checks.
func (m *Manager) SetStopper(stopper func(context.Context, string, string, string) error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.stopper = stopper
}

// Reserve claims a device for a session. Idempotent if the same session
// already owns the device.
func (m *Manager) Reserve(ctx context.Context, platform, target, sessionID string) error {
	_ = ctx
	key := sessionKey{platform: platform, target: target}
	m.mu.Lock()
	defer m.mu.Unlock()

	owner, ok := m.reservations[key]
	if ok && owner != sessionID {
		return &ToolError{
			Code: contract.ErrDeviceReserved,
			Msg:  fmt.Sprintf("device reserved by session %s", owner),
		}
	}

	m.reservations[key] = sessionID
	m.activity[key] = m.now()
	return nil
}

// CheckAccess returns an error if the device is reserved by another session.
func (m *Manager) CheckAccess(platform, target, sessionID string) error {
	key := sessionKey{platform: platform, target: target}
	m.mu.Lock()
	defer m.mu.Unlock()

	owner, ok := m.reservations[key]
	if ok && owner != sessionID {
		return &ToolError{
			Code: contract.ErrDeviceReserved,
			Msg:  fmt.Sprintf("device reserved by session %s", owner),
		}
	}
	return nil
}

// RecordActivity updates the last-activity timestamp for a reserved device.
func (m *Manager) RecordActivity(platform, target string) {
	key := sessionKey{platform: platform, target: target}
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.reservations[key]; ok {
		m.activity[key] = m.now()
	}
}

// Release removes a reservation and its activity tracking.
func (m *Manager) Release(platform, target string) {
	key := sessionKey{platform: platform, target: target}
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.reservations, key)
	delete(m.activity, key)
}

// Owner returns the session that owns a device, if any.
func (m *Manager) Owner(platform, target string) (string, bool) {
	key := sessionKey{platform: platform, target: target}
	m.mu.Lock()
	defer m.mu.Unlock()
	owner, ok := m.reservations[key]
	return owner, ok
}

// Start runs the idle sweeper until ctx is cancelled.
func (m *Manager) Start(ctx context.Context) {
	if m.timeout <= 0 {
		return
	}

	interval := m.timeout / 2
	if interval < time.Minute {
		interval = time.Minute
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			m.sweep(ctx)
		}
	}
}

type idleItem struct {
	key   sessionKey
	owner string
}

func (m *Manager) sweep(ctx context.Context) {
	now := m.now()
	m.mu.Lock()
	var idle []idleItem
	for key, t := range m.activity {
		if now.Sub(t) > m.timeout {
			idle = append(idle, idleItem{key: key, owner: m.reservations[key]})
		}
	}
	m.mu.Unlock()

	for _, it := range idle {
		m.logger.Info("stopping idle device",
			"platform", it.key.platform,
			"target", it.key.target,
			"owner", it.owner)

		if m.stopper != nil {
			if err := m.stopper(ctx, it.key.platform, it.key.target, it.owner); err != nil {
				m.logger.Error("failed to stop idle device", "platform", it.key.platform, "target", it.key.target, "error", err)
			}
		}

		m.mu.Lock()
		if m.reservations[it.key] == it.owner {
			delete(m.reservations, it.key)
			delete(m.activity, it.key)
		}
		m.mu.Unlock()
	}
}
