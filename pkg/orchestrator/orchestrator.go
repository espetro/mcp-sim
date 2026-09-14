// Package orchestrator owns device/controller lifecycle coordination across
// registered platforms. It imports only pkg/contract and the standard library.
//
// Behavioral invariants:
//
//   - Actions are idempotent: Boot on a running device converges (profile
//     reconciliation, then success), Stop on a stopped device is a no-op.
//   - Reads degrade gracefully: List across platforms returns partial results
//     when some platforms fail; only total failure is an error.
//   - Wipe returns a device to its configured baseline: after Platform.Wipe,
//     the optional ReconcileProfile hook re-applies the platform profile.
package orchestrator

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/espetro/mcp-sim/pkg/contract"
)

// profileReconciler is an optional interface a Platform may implement to
// participate in wipe/boot profile reconciliation. Implementations MUST be
// idempotent: no redundant work if the device is already in its target state
// (e.g. iOS checks OptimizeStatus.Slimmed before invoking the optimizer).
// platforms/ios implements it as: slim enabled && on_boot => Optimize, else no-op.
type profileReconciler interface {
	ReconcileProfile(ctx context.Context, target string) error
}

const defaultBootTimeout = 60 * time.Second

// Orchestrator coordinates devices and controllers across registered platforms.
// It is safe for concurrent use.
type Orchestrator struct {
	mu          sync.RWMutex
	platforms   map[string]contract.Platform
	controllers map[string]contract.Controller
	verifier    contract.Verifier
	logger      *slog.Logger
	reconcile   bool
}

// settings is the mutable target Option functions apply to.
type settings struct {
	logger      *slog.Logger
	platforms   map[string]contract.Platform
	controllers map[string]contract.Controller
	verifier    contract.Verifier
	reconcile   bool
}

// Option configures an Orchestrator at construction time. Options are
// fallible: New returns the error instead of panicking.
type Option func(*settings) error

// WithLogger sets the logger; nil falls back to slog.Default().
func WithLogger(l *slog.Logger) Option {
	return func(s *settings) error {
		if l == nil {
			l = slog.Default()
		}
		s.logger = l
		return nil
	}
}

// WithPlatform registers a platform at construction; nil or duplicate name errors.
func WithPlatform(p contract.Platform) Option {
	return func(s *settings) error {
		if p == nil {
			return fmt.Errorf("orchestrator: nil platform")
		}
		name := p.Name()
		if _, dup := s.platforms[name]; dup {
			return fmt.Errorf("orchestrator: duplicate platform %q", name)
		}
		s.platforms[name] = p
		return nil
	}
}

// WithController registers a controller at construction; nil or duplicate name errors.
func WithController(c contract.Controller) Option {
	return func(s *settings) error {
		if c == nil {
			return fmt.Errorf("orchestrator: nil controller")
		}
		name := c.Name()
		if _, dup := s.controllers[name]; dup {
			return fmt.Errorf("orchestrator: duplicate controller %q", name)
		}
		s.controllers[name] = c
		return nil
	}
}

// WithWipePolicy toggles profile reconciliation after Wipe and on Boot.
// Defaults to true (reconcile).
func WithWipePolicy(reconcile bool) Option {
	return func(s *settings) error {
		s.reconcile = reconcile
		return nil
	}
}

// New builds an Orchestrator from options.
func New(opts ...Option) (*Orchestrator, error) {
	s := &settings{
		logger:      slog.Default(),
		platforms:   map[string]contract.Platform{},
		controllers: map[string]contract.Controller{},
		reconcile:   true,
	}
	for _, opt := range opts {
		if opt == nil {
			continue
		}
		if err := opt(s); err != nil {
			return nil, err
		}
	}
	return &Orchestrator{
		platforms:   s.platforms,
		controllers: s.controllers,
		verifier:    s.verifier,
		logger:      s.logger,
		reconcile:   s.reconcile,
	}, nil
}

// RegisterPlatform adds a platform at runtime; duplicate name errors.
func (o *Orchestrator) RegisterPlatform(p contract.Platform) error {
	if p == nil {
		return fmt.Errorf("orchestrator: nil platform")
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	name := p.Name()
	if _, dup := o.platforms[name]; dup {
		return fmt.Errorf("orchestrator: duplicate platform %q", name)
	}
	o.platforms[name] = p
	return nil
}

// RegisterController adds a controller at runtime; duplicate name errors.
func (o *Orchestrator) RegisterController(c contract.Controller) error {
	if c == nil {
		return fmt.Errorf("orchestrator: nil controller")
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	name := c.Name()
	if _, dup := o.controllers[name]; dup {
		return fmt.Errorf("orchestrator: duplicate controller %q", name)
	}
	o.controllers[name] = c
	return nil
}

// PlatformByName returns the registered platform by name.
func (o *Orchestrator) PlatformByName(name string) (contract.Platform, bool) {
	o.mu.RLock()
	defer o.mu.RUnlock()
	p, ok := o.platforms[name]
	return p, ok
}

func (o *Orchestrator) controllerByName(name string) (contract.Controller, bool) {
	o.mu.RLock()
	defer o.mu.RUnlock()
	c, ok := o.controllers[name]
	return c, ok
}

func (o *Orchestrator) allPlatforms() map[string]contract.Platform {
	o.mu.RLock()
	defer o.mu.RUnlock()
	out := make(map[string]contract.Platform, len(o.platforms))
	for k, v := range o.platforms {
		out[k] = v
	}
	return out
}

func (o *Orchestrator) allControllers() map[string]contract.Controller {
	o.mu.RLock()
	defer o.mu.RUnlock()
	out := make(map[string]contract.Controller, len(o.controllers))
	for k, v := range o.controllers {
		out[k] = v
	}
	return out
}

// reconcileProfile applies the platform's optional ReconcileProfile hook.
func (o *Orchestrator) reconcileProfile(ctx context.Context, p contract.Platform, target string) {
	if !o.reconcile {
		return
	}
	r, ok := p.(profileReconciler)
	if !ok {
		return
	}
	if err := r.ReconcileProfile(ctx, target); err != nil {
		o.logger.Warn("profile reconciliation failed", "platform", p.Name(), "target", target, "err", err)
	}
}

// List returns devices across all platforms. A failing platform is skipped
// with a warning; an error is returned only when every platform fails (an
// empty registry yields an empty result, not an error).
func (o *Orchestrator) List(ctx context.Context) ([]contract.Device, error) {
	var result []contract.Device
	var errs []error
	platforms := o.allPlatforms()
	for name, p := range platforms {
		devs, err := p.List(ctx)
		if err != nil {
			o.logger.Warn("listing devices failed", "platform", name, "err", err)
			errs = append(errs, fmt.Errorf("listing devices on %s: %w", name, err))
			continue
		}
		result = append(result, devs...)
	}
	// Only a total failure is an error; a healthy platform keeps the read alive.
	if len(platforms) > 0 && len(errs) == len(platforms) {
		return nil, fmt.Errorf("all platforms failed: %w", errors.Join(errs...))
	}
	return result, nil
}

// State returns the state of a single device. Single-platform semantics:
// errors propagate.
func (o *Orchestrator) State(ctx context.Context, platform, target string) (contract.DeviceState, error) {
	p, ok := o.PlatformByName(platform)
	if !ok {
		return contract.DeviceStateUnknown, unsupportedPlatform(platform)
	}
	state, err := p.State(ctx, target)
	if err != nil {
		return contract.DeviceStateUnknown, err
	}
	return state, nil
}

// Boot starts a device and waits for readiness. Idempotent: if the device is
// already running, the ensure-profile step runs and the current Device is
// returned as success ("already in state" is success, not ErrAlreadyRunning).
func (o *Orchestrator) Boot(ctx context.Context, platform, target string, opts contract.StartOpts) (contract.Device, error) {
	p, ok := o.PlatformByName(platform)
	if !ok {
		return contract.Device{}, unsupportedPlatform(platform)
	}

	state, err := p.State(ctx, target)
	if err != nil {
		return contract.Device{}, err
	}

	if state == contract.DeviceStateRunning {
		o.reconcileProfile(ctx, p, target)
		return o.deviceFor(ctx, p, platform, target)
	}

	dev, err := p.Start(ctx, target, opts)
	if err != nil {
		return contract.Device{}, err
	}

	timeout := opts.Timeout
	if timeout == 0 {
		timeout = defaultBootTimeout
	}
	if err := p.AwaitReady(ctx, target, timeout); err != nil {
		return dev, NewToolError(contract.ErrTimeout, "device did not become ready: "+target)
	}

	o.reconcileProfile(ctx, p, target)
	return dev, nil
}

// deviceFor looks up the current Device by ID via List.
func (o *Orchestrator) deviceFor(ctx context.Context, p contract.Platform, platform, target string) (contract.Device, error) {
	devs, err := p.List(ctx)
	if err != nil {
		return contract.Device{}, err
	}
	for _, d := range devs {
		if d.ID == target {
			return d, nil
		}
	}
	return contract.Device{}, NewToolError(contract.ErrDeviceNotFound, "device not found: "+target)
}

// Stop stops a device. Idempotent: stopping a stopped device returns nil
// without touching the adapter.
func (o *Orchestrator) Stop(ctx context.Context, platform, target string) error {
	p, ok := o.PlatformByName(platform)
	if !ok {
		return unsupportedPlatform(platform)
	}
	state, err := p.State(ctx, target)
	if err != nil {
		return err
	}
	if state == contract.DeviceStateStopped {
		return nil
	}
	return p.Stop(ctx, target)
}

// Wipe erases a device and returns it to its configured baseline: if the
// platform implements profile reconciliation and the wipe policy is active,
// ReconcileProfile runs after the adapter wipe.
func (o *Orchestrator) Wipe(ctx context.Context, platform, target string) error {
	p, ok := o.PlatformByName(platform)
	if !ok {
		return unsupportedPlatform(platform)
	}
	state, err := p.State(ctx, target)
	if err != nil {
		return err
	}
	if state == contract.DeviceStateRunning {
		if err := p.Stop(ctx, target); err != nil {
			return err
		}
	}
	if err := p.Wipe(ctx, target); err != nil {
		return err
	}
	if o.reconcile {
		if r, can := p.(profileReconciler); can {
			if err := r.ReconcileProfile(ctx, target); err != nil {
				return fmt.Errorf("profile reconciliation after wipe: %w", err)
			}
		}
	}
	return nil
}

// AwaitReady waits for a device to become ready.
func (o *Orchestrator) AwaitReady(ctx context.Context, platform, target string, timeout time.Duration) error {
	p, ok := o.PlatformByName(platform)
	if !ok {
		return unsupportedPlatform(platform)
	}
	return p.AwaitReady(ctx, target, timeout)
}

// OpenURL opens a deep link on a device.
func (o *Orchestrator) OpenURL(ctx context.Context, platform, target, url string) error {
	p, ok := o.PlatformByName(platform)
	if !ok {
		return unsupportedPlatform(platform)
	}
	return p.OpenURL(ctx, target, url)
}

// OptimizerState returns optimizer status and live resource usage for a
// device. Capability-driven: platforms without CapOptimize return
// (nil, nil, nil).
func (o *Orchestrator) OptimizerState(ctx context.Context, platform, target string) (*contract.OptimizeStatus, *contract.ResourceUsage, error) {
	p, ok := o.PlatformByName(platform)
	if !ok {
		return nil, nil, unsupportedPlatform(platform)
	}
	if !p.Capabilities().Has(contract.CapOptimize) {
		return nil, nil, nil
	}
	opt, ok := contract.As[contract.Optimizer](p)
	if !ok {
		return nil, nil, nil
	}
	st, err := opt.OptimizeStatus(ctx, target)
	if err != nil {
		return nil, nil, err
	}
	var usage *contract.ResourceUsage
	// Measure only works on a booted device; ignore errors rather than
	// failing the read.
	if state, err := p.State(ctx, target); err == nil && state == contract.DeviceStateRunning {
		if m, err := opt.Measure(ctx, target); err == nil {
			usage = &m
		}
	}
	return &st, usage, nil
}

// StartController starts a controller by name.
func (o *Orchestrator) StartController(ctx context.Context, name string, cfg contract.StartConfig) (contract.ProxyInfo, error) {
	c, ok := o.controllerByName(name)
	if !ok {
		return contract.ProxyInfo{}, unsupportedController(name)
	}
	return c.Start(ctx, cfg)
}

// StopController stops a controller by name.
func (o *Orchestrator) StopController(ctx context.Context, name string) error {
	c, ok := o.controllerByName(name)
	if !ok {
		return unsupportedController(name)
	}
	return c.Stop(ctx)
}

// ControllerStatus returns controller status by name.
func (o *Orchestrator) ControllerStatus(ctx context.Context, name string) (contract.ProxyInfo, error) {
	c, ok := o.controllerByName(name)
	if !ok {
		return contract.ProxyInfo{}, unsupportedController(name)
	}
	return c.Status(ctx)
}

// ShutdownAll stops all running devices and all controllers. Errors are
// collected and joined; it always attempts every shutdown.
func (o *Orchestrator) ShutdownAll(ctx context.Context) error {
	var errs []error
	for name, p := range o.allPlatforms() {
		devs, err := p.List(ctx)
		if err != nil {
			errs = append(errs, fmt.Errorf("listing devices on %s during shutdown: %w", name, err))
			continue
		}
		for _, d := range devs {
			if d.State == contract.DeviceStateRunning {
				if err := p.Stop(ctx, d.ID); err != nil {
					errs = append(errs, fmt.Errorf("stopping device %s on %s: %w", d.ID, name, err))
				}
			}
		}
	}
	for name, c := range o.allControllers() {
		if err := c.Stop(ctx); err != nil {
			errs = append(errs, fmt.Errorf("stopping controller %s: %w", name, err))
		}
	}
	return errors.Join(errs...)
}
