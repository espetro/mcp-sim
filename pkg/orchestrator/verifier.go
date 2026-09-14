package orchestrator

import (
	"context"
	"errors"

	"github.com/espetro/mcp-sim/pkg/contract"
)

// WithVerifier registers the device-interaction backend used by the
// verifier.* tools. Nil or duplicate registration errors. Absence is valid:
// verifier tools degrade to verifier_unavailable ToolErrors.
func WithVerifier(v contract.Verifier) Option {
	return func(s *settings) error {
		if v == nil {
			return errNilVerifier
		}
		s.verifier = v
		return nil
	}
}

// errNilVerifier is returned for a nil WithVerifier option.
var errNilVerifier = errors.New("orchestrator: nil verifier")

// Verifier returns the registered verifier, if any.
func (o *Orchestrator) Verifier() (contract.Verifier, bool) {
	o.mu.RLock()
	defer o.mu.RUnlock()
	if o.verifier == nil {
		return nil, false
	}
	return o.verifier, true
}

// verifierOrErr returns the registered verifier or a verifier_unavailable
// ToolError when none is configured.
func (o *Orchestrator) verifierOrErr() (contract.Verifier, error) {
	if v, ok := o.Verifier(); ok {
		return v, nil
	}
	return nil, NewToolError(contract.ErrVerifierUnavailable,
		"no verifier backend configured: enable controllers.agentdevice.verifier or install agent-device")
}

// VerifyTap taps an element identified by a snapshot ref.
func (o *Orchestrator) VerifyTap(ctx context.Context, target, ref string) error {
	v, err := o.verifierOrErr()
	if err != nil {
		return err
	}
	return v.Tap(ctx, target, ref)
}

// VerifyDoubleTap double-taps an element identified by a snapshot ref.
func (o *Orchestrator) VerifyDoubleTap(ctx context.Context, target, ref string) error {
	v, err := o.verifierOrErr()
	if err != nil {
		return err
	}
	return v.DoubleTap(ctx, target, ref)
}

// VerifyLongPress long-presses an element identified by a snapshot ref.
func (o *Orchestrator) VerifyLongPress(ctx context.Context, target, ref string, ms int) error {
	v, err := o.verifierOrErr()
	if err != nil {
		return err
	}
	return v.LongPress(ctx, target, ref, ms)
}

// VerifySwipe swipes the screen in a direction.
func (o *Orchestrator) VerifySwipe(ctx context.Context, target, direction string) error {
	v, err := o.verifierOrErr()
	if err != nil {
		return err
	}
	return v.Swipe(ctx, target, direction)
}

// VerifyType enters text into an element identified by a snapshot ref.
func (o *Orchestrator) VerifyType(ctx context.Context, target, ref, text string) error {
	v, err := o.verifierOrErr()
	if err != nil {
		return err
	}
	return v.Type(ctx, target, ref, text)
}

// VerifyPressButton presses a hardware/system button.
func (o *Orchestrator) VerifyPressButton(ctx context.Context, target, button string) error {
	v, err := o.verifierOrErr()
	if err != nil {
		return err
	}
	return v.PressButton(ctx, target, button)
}

// VerifyOpenURL opens a URL or deep link via the verifier backend.
func (o *Orchestrator) VerifyOpenURL(ctx context.Context, target, url string) error {
	v, err := o.verifierOrErr()
	if err != nil {
		return err
	}
	return v.OpenURL(ctx, target, url)
}

// VerifySnapshot captures the current screen hierarchy.
func (o *Orchestrator) VerifySnapshot(ctx context.Context, target string) (contract.Snapshot, error) {
	v, err := o.verifierOrErr()
	if err != nil {
		return contract.Snapshot{}, err
	}
	return v.Snapshot(ctx, target)
}

// VerifyScreenshot captures the current screen image.
func (o *Orchestrator) VerifyScreenshot(ctx context.Context, target string) (contract.Screenshot, error) {
	v, err := o.verifierOrErr()
	if err != nil {
		return contract.Screenshot{}, err
	}
	return v.Screenshot(ctx, target)
}
