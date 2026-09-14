package android

import (
	"context"
	"fmt"
	"strconv"
	"strings"
)

// InstallApp installs an APK on an emulator/device (`adb install -r`).
// Implements contract.AppInstaller.
func (p *Platform) InstallApp(ctx context.Context, target, artifactPath string) error {
	serial, err := p.serialFor(ctx, target)
	if err != nil {
		return err
	}
	cmd := p.adbCmd(ctx, "-s", serial, "install", "-r", artifactPath)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("adb -s %s install -r %s: %w\n%s", serial, artifactPath, err, out)
	}
	return nil
}

// LaunchApp launches an installed app: the launch activity is resolved via
// `cmd package resolve-activity --brief`, then started via `am start -n`.
// The pid is read back with `pidof` (0 when unavailable).
// Implements contract.AppLauncher.
func (p *Platform) LaunchApp(ctx context.Context, target, bundleID string) (int, error) {
	serial, err := p.serialFor(ctx, target)
	if err != nil {
		return 0, err
	}

	resolveOut, err := p.adbCmd(ctx, "-s", serial, "shell",
		"cmd", "package", "resolve-activity", "--brief", bundleID).CombinedOutput()
	if err != nil {
		return 0, fmt.Errorf("resolve-activity %s: %w\n%s", bundleID, err, resolveOut)
	}
	// Output lines end with "packageName/fully.qualified.ActivityName".
	activity := lastNonEmptyLine(string(resolveOut))
	if activity == "" || !strings.Contains(activity, "/") {
		return 0, fmt.Errorf("resolve-activity %s: no launch activity in output %q", bundleID, resolveOut)
	}

	if out, err := p.adbCmd(ctx, "-s", serial, "shell",
		"am", "start", "-n", activity).CombinedOutput(); err != nil {
		return 0, fmt.Errorf("am start -n %s: %w\n%s", activity, err, out)
	}

	pid := 0
	if pidOut, err := p.adbCmd(ctx, "-s", serial, "shell", "pidof", bundleID).Output(); err == nil {
		pid, _ = strconv.Atoi(strings.TrimSpace(string(pidOut)))
	}
	return pid, nil
}

// serialFor maps a target (AVD name) to an adb serial. Emulators started by
// this process have a port mapping; otherwise the target is used as-is so
// physical/already-running devices ("emulator-5554", a USB serial) work.
func (p *Platform) serialFor(ctx context.Context, target string) (string, error) {
	p.portMu.Lock()
	port, ok := p.avdPortMap[target]
	p.portMu.Unlock()
	if ok {
		return fmt.Sprintf("emulator-%d", port), nil
	}
	return target, nil
}

func lastNonEmptyLine(s string) string {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		if line := strings.TrimSpace(lines[i]); line != "" {
			return line
		}
	}
	return ""
}
