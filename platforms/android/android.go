package android

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/espetro/mcp-sim/internal/config"
	"github.com/espetro/mcp-sim/pkg/contract"
)

// Platform implements contract.Platform for Android Emulators via adb/emulator.
type Platform struct {
	androidHome string
	javaHome   string
	emulatorBin string
	avdPortMap  map[string]int // AVD name → port
}

// New creates a new Android platform adapter.
func New(cfg config.AndroidConfig) (*Platform, error) {
	androidHome := cfg.AndroidHome
	if androidHome == "" {
		androidHome = os.Getenv("ANDROID_HOME")
		if androidHome == "" {
			if home, _ := os.UserHomeDir(); home != "" {
				androidHome = home + "/Library/Android/sdk"
				if _, err := os.Stat(androidHome); os.IsNotExist(err) {
					androidHome = home + "/Android/Sdk"
				}
			}
		}
	}

	javaHome := cfg.JavaHome
	if javaHome == "" {
		javaHome = os.Getenv("JAVA_HOME")
	}

	emulatorBin := cfg.EmulatorBin
	if emulatorBin == "" {
		emulatorBin = "emulator"
	}

	return &Platform{
		androidHome: androidHome,
		javaHome:   javaHome,
		emulatorBin: emulatorBin,
		avdPortMap:  make(map[string]int),
	}, nil
}

// Name returns "android".
func (p *Platform) Name() string { return "android" }

func (p *Platform) env() []string {
	env := os.Environ()
	if p.androidHome != "" {
		env = append(env, "ANDROID_HOME="+p.androidHome)
	}
	if p.javaHome != "" {
		env = append(env, "JAVA_HOME="+p.javaHome)
	}
	return env
}

func (p *Platform) emulatorCmd(ctx context.Context, args ...string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, p.emulatorBin, args...)
	cmd.Env = p.env()
	return cmd
}

func (p *Platform) adbCmd(ctx context.Context, args ...string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, "adb", args...)
	cmd.Env = p.env()
	return cmd
}

// ListAVDs returns AVD names via `emulator -list-avds`.
func (p *Platform) ListAVDs(ctx context.Context) ([]string, error) {
	cmd := p.emulatorCmd(ctx, "-list-avds")
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("emulator -list-avds: %w", err)
	}
	var avds []string
	sc := bufio.NewScanner(bytes.NewReader(out))
	for sc.Scan() {
		name := strings.TrimSpace(sc.Text())
		if name != "" {
			avds = append(avds, name)
		}
	}
	return avds, nil
}

// List returns all Android emulators.
func (p *Platform) List(ctx context.Context) ([]contract.Device, error) {
	avds, err := p.ListAVDs(ctx)
	if err != nil {
		return nil, err
	}

	adbOut, err := p.adbCmd(ctx, "devices").Output()
	if err != nil {
		return nil, fmt.Errorf("adb devices: %w", err)
	}

	runningSerials := make(map[string]bool)
	sc := bufio.NewScanner(bytes.NewReader(adbOut))
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "List") {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) >= 2 && parts[1] == "device" {
			runningSerials[parts[0]] = true
		}
	}

	var devs []contract.Device
	for _, name := range avds {
		port, ok := p.avdPortMap[name]
		state := contract.DeviceStateStopped
		if ok {
			serial := fmt.Sprintf("emulator-%d", port)
			if runningSerials[serial] {
				state = contract.DeviceStateRunning
			}
		}
		devs = append(devs, contract.Device{
			ID:       name,
			Name:     name,
			Platform: "android",
			State:    state,
		})
	}
	return devs, nil
}

// Start launches an emulator for the given AVD.
// Uses SysProcAttr{Setpgid:true} so the emulator survives parent death.
func (p *Platform) Start(ctx context.Context, target string, opts contract.StartOpts) (contract.Device, error) {
	port := opts.Port
	if port == 0 {
		// Find a free port. Start at 5554 (even ports are console, odd are adb).
		port = 5554
	}

	args := []string{"-avd", target, "-port", strconv.Itoa(port), "-no-snapshot-load"}
	if opts.NoWindow {
		args = append(args, "-no-window")
	}

	cmd := p.emulatorCmd(ctx, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := cmd.Start(); err != nil {
		return contract.Device{}, fmt.Errorf("emulator start: %w", err)
	}

	p.avdPortMap[target] = port
	serial := fmt.Sprintf("emulator-%d", port)

	// Wait for adb to register the device.
	deadline, cancel := context.WithTimeout(ctx, 90*time.Second)
	defer cancel()

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-deadline.Done():
			return contract.Device{ID: target, Name: target, Platform: "android", State: contract.DeviceStateBooting}, nil
		case <-ticker.C:
			out, _ := p.adbCmd(deadline, "-s", serial, "get-state").Output()
			if strings.TrimSpace(string(out)) == "device" {
				return contract.Device{ID: target, Name: target, Platform: "android", State: contract.DeviceStateRunning}, nil
			}
		}
	}
}

// Stop stops an emulator.
func (p *Platform) Stop(ctx context.Context, target string) error {
	port, ok := p.avdPortMap[target]
	if !ok {
		return fmt.Errorf("no port mapping for AVD: %s", target)
	}
	serial := fmt.Sprintf("emulator-%d", port)
	_ = p.adbCmd(ctx, "-s", serial, "emu", "kill").Run()
	return nil
}

// State returns the state of an emulator.
func (p *Platform) State(ctx context.Context, target string) (contract.DeviceState, error) {
	port, ok := p.avdPortMap[target]
	if !ok {
		// Try to discover port from adb devices.
		out, err := p.adbCmd(ctx, "devices").Output()
		if err != nil {
			return contract.DeviceStateUnknown, err
		}
		sc := bufio.NewScanner(bytes.NewReader(out))
		for sc.Scan() {
			line := strings.TrimSpace(sc.Text())
			if line == "" || strings.HasPrefix(line, "List") {
				continue
			}
			parts := strings.Fields(line)
			if len(parts) < 2 || parts[1] != "device" {
				continue
			}
			serial := parts[0]
			if strings.HasPrefix(serial, "emulator-") {
				if n, err := strconv.Atoi(strings.TrimPrefix(serial, "emulator-")); err == nil {
					p.avdPortMap[target] = n
					port = n
					break
				}
			}
		}
		if port == 0 {
			return contract.DeviceStateStopped, nil
		}
	}

	serial := fmt.Sprintf("emulator-%d", port)
	out, err := p.adbCmd(ctx, "-s", serial, "get-state").Output()
	if err != nil {
		return contract.DeviceStateUnknown, err
	}
	switch strings.TrimSpace(string(out)) {
	case "device":
		return contract.DeviceStateRunning, nil
	case "offline":
		return contract.DeviceStateStopped, nil
	default:
		return contract.DeviceStateUnknown, nil
	}
}

// AwaitReady polls until the emulator is responsive.
func (p *Platform) AwaitReady(ctx context.Context, target string, timeout time.Duration) error {
	deadline, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-deadline.Done():
			return fmt.Errorf("await ready timeout for %s", target)
		case <-ticker.C:
			state, err := p.State(ctx, target)
			if err == nil && state == contract.DeviceStateRunning {
				return nil
			}
		}
	}
}

// Wipe wipes the emulator user data.
func (p *Platform) Wipe(ctx context.Context, target string) error {
	_ = p.Stop(ctx, target)
	// Restart with -wipe-data.
	cmd := p.emulatorCmd(ctx, "-avd", target, "-wipe-data")
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	_ = cmd.Start()
	return nil
}

// OpenURL opens a deep link on the emulator.
func (p *Platform) OpenURL(ctx context.Context, target, url string) error {
	port, ok := p.avdPortMap[target]
	if !ok {
		return fmt.Errorf("no port mapping for AVD: %s", target)
	}
	serial := fmt.Sprintf("emulator-%d", port)
	cmd := p.adbCmd(ctx, "-s", serial, "shell", "am", "start", "-a", "android.intent.action.VIEW", "-d", url)
	return cmd.Run()
}
