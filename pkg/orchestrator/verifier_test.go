package orchestrator

import (
	"context"
	"errors"
	"testing"

	"github.com/espetro/mcp-sim/pkg/contract"
)

// fakeVerifier records invocations for assertions.
type fakeVerifier struct {
	tapTarget, tapRef     string
	swipeTarget, swipeDir string
	typeTarget, typeRef   string
	typeText              string
	buttonTarget, button  string
	urlTarget, url        string
	snapTarget            string
	snapshot              contract.Snapshot
	screenshot            contract.Screenshot
	screenshotErr         error
	pingErr               error
}

func (f *fakeVerifier) Name() string { return "fake" }

func (f *fakeVerifier) Ping(_ context.Context) error { return f.pingErr }

func (f *fakeVerifier) Tap(_ context.Context, target, ref string) error {
	f.tapTarget, f.tapRef = target, ref
	return nil
}

func (f *fakeVerifier) DoubleTap(_ context.Context, target, ref string) error {
	f.tapTarget, f.tapRef = "double:"+target, ref
	return nil
}

func (f *fakeVerifier) LongPress(_ context.Context, target, ref string, _ int) error {
	f.tapTarget, f.tapRef = "long:"+target, ref
	return nil
}

func (f *fakeVerifier) Swipe(_ context.Context, target, direction string) error {
	f.swipeTarget, f.swipeDir = target, direction
	return nil
}

func (f *fakeVerifier) Type(_ context.Context, target, ref, text string) error {
	f.typeTarget, f.typeRef, f.typeText = target, ref, text
	return nil
}

func (f *fakeVerifier) PressButton(_ context.Context, target, button string) error {
	f.buttonTarget, f.button = target, button
	return nil
}

func (f *fakeVerifier) OpenURL(_ context.Context, target, url string) error {
	f.urlTarget, f.url = target, url
	return nil
}

func (f *fakeVerifier) Snapshot(_ context.Context, target string) (contract.Snapshot, error) {
	f.snapTarget = target
	return f.snapshot, nil
}

func (f *fakeVerifier) Screenshot(_ context.Context, target string) (contract.Screenshot, error) {
	if f.screenshotErr != nil {
		return contract.Screenshot{}, f.screenshotErr
	}
	return f.screenshot, nil
}

func newOrchestratorWithVerifier(t *testing.T) (*Orchestrator, *fakeVerifier) {
	t.Helper()
	v := &fakeVerifier{snapshot: contract.Snapshot{
		Platform: "ios",
		Target:   "ios-dev",
		Nodes: []contract.SnapshotNode{
			{Ref: "n1", Role: "Button", Label: "Share", Enabled: true},
		},
	}}
	o, err := New(WithVerifier(v))
	if err != nil {
		t.Fatal(err)
	}
	return o, v
}

func TestWithVerifierNil(t *testing.T) {
	if _, err := New(WithVerifier(nil)); err == nil {
		t.Fatal("want error for nil verifier, got nil")
	}
}

func TestVerifierTapDispatch(t *testing.T) {
	o, v := newOrchestratorWithVerifier(t)

	if err := o.VerifyTap(context.Background(), "ios-dev", "n1"); err != nil {
		t.Fatalf("VerifyTap: %v", err)
	}
	if v.tapTarget != "ios-dev" || v.tapRef != "n1" {
		t.Errorf("adapter got target=%q ref=%q, want ios-dev/n1", v.tapTarget, v.tapRef)
	}
}

func TestVerifierTypeDispatch(t *testing.T) {
	o, v := newOrchestratorWithVerifier(t)

	if err := o.VerifyType(context.Background(), "emulator-5554", "n2", "hello"); err != nil {
		t.Fatalf("VerifyType: %v", err)
	}
	if v.typeTarget != "emulator-5554" || v.typeRef != "n2" || v.typeText != "hello" {
		t.Errorf("adapter got target=%q ref=%q text=%q", v.typeTarget, v.typeRef, v.typeText)
	}
}

func TestVerifierSnapshotDispatch(t *testing.T) {
	o, _ := newOrchestratorWithVerifier(t)

	snap, err := o.VerifySnapshot(context.Background(), "ios-dev")
	if err != nil {
		t.Fatalf("VerifySnapshot: %v", err)
	}
	if snap.Target != "ios-dev" || len(snap.Nodes) != 1 || snap.Nodes[0].Label != "Share" {
		t.Errorf("unexpected snapshot: %+v", snap)
	}
}

func TestVerifierScreenshotPropagatesError(t *testing.T) {
	o, v := newOrchestratorWithVerifier(t)
	v.screenshotErr = errors.New("screen capture failed")

	if _, err := o.VerifyScreenshot(context.Background(), "ios-dev"); err == nil {
		t.Fatal("want error from adapter, got nil")
	}
}

func TestVerifierUnavailableWithoutBackend(t *testing.T) {
	o, err := New()
	if err != nil {
		t.Fatal(err)
	}

	if err := o.VerifyTap(context.Background(), "ios-dev", "n1"); err == nil {
		t.Fatal("want verifier_unavailable, got nil")
	} else {
		var te *ToolError
		if !errors.As(err, &te) || te.Code != contract.ErrVerifierUnavailable {
			t.Errorf("want verifier_unavailable ToolError, got %v", err)
		}
	}
	if _, err := o.VerifySnapshot(context.Background(), "x"); err == nil {
		t.Error("want verifier_unavailable for snapshot, got nil")
	}
	if _, err := o.VerifyScreenshot(context.Background(), "x"); err == nil {
		t.Error("want verifier_unavailable for screenshot, got nil")
	}
}

func TestVerifierAccessor(t *testing.T) {
	o, _ := newOrchestratorWithVerifier(t)
	got, ok := o.Verifier()
	if !ok || got.Name() != "fake" {
		t.Errorf("Verifier() = %v, %v; want fake verifier", got, ok)
	}

	empty, err := New()
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := empty.Verifier(); ok {
		t.Error("empty orchestrator should have no verifier")
	}
}
