package ios

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
)

// InstallApp installs an app bundle on a simulator (`simctl install`).
// Implements contract.AppInstaller.
func (p *Platform) InstallApp(ctx context.Context, target, artifactPath string) error {
	cmd := p.xcrun(ctx, "simctl", "install", target, artifactPath)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("simctl install %s %s: %w\n%s", target, artifactPath, err, out)
	}
	return nil
}

// launchPidRe parses the pid out of `simctl launch` output, which looks like
// "com.bundle.id: 1234".
var launchPidRe = regexp.MustCompile(`:\s*(\d+)\s*$`)

// LaunchApp launches an installed app and returns its pid.
// Implements contract.AppLauncher.
func (p *Platform) LaunchApp(ctx context.Context, target, bundleID string) (int, error) {
	out, err := p.xcrun(ctx, "simctl", "launch", target, bundleID).CombinedOutput()
	if err != nil {
		return 0, fmt.Errorf("simctl launch %s %s: %w\n%s", target, bundleID, err, out)
	}
	m := launchPidRe.FindStringSubmatch(string(out))
	if m == nil {
		return 0, fmt.Errorf("simctl launch %s: could not parse pid from output %q", bundleID, out)
	}
	pid, err := strconv.Atoi(m[1])
	if err != nil {
		return 0, fmt.Errorf("simctl launch %s: parse pid: %w", bundleID, err)
	}
	return pid, nil
}
