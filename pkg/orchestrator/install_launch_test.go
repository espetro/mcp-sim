package orchestrator

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/espetro/mcp-sim/pkg/contract"
)

// fakeAppPlatform is a fakePlatform that also implements AppInstaller and
// AppLauncher, recording invocations.
type fakeAppPlatform struct {
	*fakePlatform

	installPath string
	launchID    string
	launchPID   int
	installErr  error
	launchErr   error
}

func (f *fakeAppPlatform) Capabilities() contract.CapabilitySet {
	return contract.CapabilitiesFor(f)
}

func (f *fakeAppPlatform) InstallApp(_ context.Context, target, artifactPath string) error {
	f.calls = append(f.calls, "InstallApp:"+target+":"+artifactPath)
	if f.installErr != nil {
		return f.installErr
	}
	f.installPath = artifactPath
	return nil
}

func (f *fakeAppPlatform) LaunchApp(_ context.Context, target, bundleID string) (int, error) {
	f.calls = append(f.calls, "LaunchApp:"+target+":"+bundleID)
	if f.launchErr != nil {
		return 0, f.launchErr
	}
	f.launchID = bundleID
	return f.launchPID, nil
}

func newOrchestratorWithAppPlatform(t *testing.T) (*Orchestrator, *fakeAppPlatform) {
	t.Helper()
	o, err := New(WithPlatform(&fakeAppPlatform{
		fakePlatform: newFake("ios", contract.DeviceStateRunning),
		launchPID:    4321,
	}))
	if err != nil {
		t.Fatal(err)
	}
	p := o.platforms["ios"].(*fakeAppPlatform)
	return o, p
}

func TestInstallApp(t *testing.T) {
	o, p := newOrchestratorWithAppPlatform(t)

	err := o.InstallApp(context.Background(), "ios", "ios-dev", "/tmp/MyApp.app")
	if err != nil {
		t.Fatalf("InstallApp: %v", err)
	}
	if p.installPath != "/tmp/MyApp.app" {
		t.Errorf("adapter got path %q, want /tmp/MyApp.app", p.installPath)
	}
}

func TestInstallAppUnsupportedPlatform(t *testing.T) {
	o, _ := newOrchestratorWithAppPlatform(t)

	err := o.InstallApp(context.Background(), "android", "dev", "/tmp/x.apk")
	var te *ToolError
	if !errors.As(err, &te) || te.Code != contract.ErrUnsupportedPlatform {
		t.Errorf("want unsupported_platform ToolError, got %v", err)
	}
}

func TestInstallAppPropagatesAdapterError(t *testing.T) {
	o, p := newOrchestratorWithAppPlatform(t)
	p.installErr = errors.New("no space left on device")

	if err := o.InstallApp(context.Background(), "ios", "ios-dev", "/tmp/x.app"); err == nil {
		t.Fatal("want error from adapter, got nil")
	}
}

func TestLaunchApp(t *testing.T) {
	o, p := newOrchestratorWithAppPlatform(t)

	pid, err := o.LaunchApp(context.Background(), "ios", "ios-dev", "com.example.hello")
	if err != nil {
		t.Fatalf("LaunchApp: %v", err)
	}
	if pid != 4321 {
		t.Errorf("pid = %d, want 4321", pid)
	}
	if p.launchID != "com.example.hello" {
		t.Errorf("adapter got bundle id %q", p.launchID)
	}
}

func TestLaunchAppUnsupportedPlatform(t *testing.T) {
	o, _ := newOrchestratorWithAppPlatform(t)

	if _, err := o.LaunchApp(context.Background(), "android", "dev", "com.x"); err == nil {
		t.Fatal("want error for unregistered platform, got nil")
	} else if fmt.Sprintf("%v", err) == "" {
		t.Fatal("empty error")
	}
}

func TestLaunchAppPropagatesAdapterError(t *testing.T) {
	o, p := newOrchestratorWithAppPlatform(t)
	p.launchErr = errors.New("bundle not installed")

	if _, err := o.LaunchApp(context.Background(), "ios", "ios-dev", "com.x"); err == nil {
		t.Fatal("want error from adapter, got nil")
	}
}
