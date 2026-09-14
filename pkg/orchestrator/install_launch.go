package orchestrator

import (
	"context"

	"github.com/espetro/mcp-sim/pkg/contract"
)

// InstallApp installs an app artifact on a device. Capability-driven:
// platforms that do not implement AppInstaller return
// unsupported_platform.
func (o *Orchestrator) InstallApp(ctx context.Context, platform, target, artifactPath string) error {
	p, ok := o.PlatformByName(platform)
	if !ok {
		return unsupportedPlatform(platform)
	}
	if !p.Capabilities().Has(contract.CapInstall) {
		return NewToolError(contract.ErrUnsupportedPlatform, "platform does not support install_app: "+platform)
	}
	inst, ok := contract.As[contract.AppInstaller](p)
	if !ok {
		return NewToolError(contract.ErrUnsupportedPlatform, "platform does not support install_app: "+platform)
	}
	return inst.InstallApp(ctx, target, artifactPath)
}

// LaunchApp starts an installed app by bundle identifier and returns its
// process id (0 when the platform cannot report one). Capability-driven:
// platforms that do not implement AppLauncher return unsupported_platform.
func (o *Orchestrator) LaunchApp(ctx context.Context, platform, target, bundleID string) (int, error) {
	p, ok := o.PlatformByName(platform)
	if !ok {
		return 0, unsupportedPlatform(platform)
	}
	if !p.Capabilities().Has(contract.CapLaunch) {
		return 0, NewToolError(contract.ErrUnsupportedPlatform, "platform does not support launch_app: "+platform)
	}
	launcher, ok := contract.As[contract.AppLauncher](p)
	if !ok {
		return 0, NewToolError(contract.ErrUnsupportedPlatform, "platform does not support launch_app: "+platform)
	}
	return launcher.LaunchApp(ctx, target, bundleID)
}
