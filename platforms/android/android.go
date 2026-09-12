package android

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/espetro/mcp-sim/internal/config"
	"github.com/espetro/mcp-sim/pkg/contract"
)

// Platform implements contract.Platform for Android Emulators via adb/emulator.
type Platform struct {
	androidHome string
	javaHome    string
	emulatorBin string
	avdPortMap  map[string]int // AVD name → port
	portMu      sync.Mutex     // guards avdPortMap (MCP tool calls run concurrently)

	// ATD (Automated Test Device) settings from config.AndroidConfig.
	imageTag      string
	api           int
	abi           string
	ramSizeMB     int
	heapSizeMB    int
	autoProvision bool

	// avdTagCache caches per-AVD ATD annotations (List may be called
	// concurrently by the orchestrator).
	tagMu       sync.Mutex
	avdTagCache map[string]tagInfo
}

// tagInfo is the cached ATD annotation for an AVD.
type tagInfo struct {
	atd      bool
	estRAMMB int
}

// New creates a new Android platform adapter.
//
// Resilient to missing Android SDK: returns (nil, nil) when neither the
// emulator nor adb binary can be located. Callers should treat a nil result
// as "skip Android registration" rather than a fatal error.
func New(cfg config.AndroidConfig) (*Platform, error) {
	// First check: does the emulator binary exist (in PATH or explicit path)?
	if cfg.EmulatorBin == "" {
		if _, err := exec.LookPath("emulator"); err != nil {
			// Try adb as a fallback signal — adb alone without an emulator AVD
			// is still useful for inspecting already-running devices.
			if _, err2 := exec.LookPath("adb"); err2 != nil {
				return nil, nil
			}
		}
	}

	androidHome := cfg.AndroidHome
	if androidHome == "" {
		androidHome = os.Getenv("ANDROID_HOME")
		if androidHome == "" {
			if home, _ := os.UserHomeDir(); home != "" {
				switch runtime.GOOS {
				case "darwin":
					androidHome = filepath.Join(home, "Library", "Android", "sdk")
				case "windows":
					if localAppData := os.Getenv("LOCALAPPDATA"); localAppData != "" {
						androidHome = filepath.Join(localAppData, "Android", "Sdk")
					} else {
						androidHome = filepath.Join(home, "AppData", "Local", "Android", "Sdk")
					}
				default:
					androidHome = filepath.Join(home, "Android", "Sdk")
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
		androidHome:   androidHome,
		javaHome:      javaHome,
		emulatorBin:   emulatorBin,
		avdPortMap:    make(map[string]int),
		imageTag:      cfg.ImageTag,
		api:           cfg.API,
		abi:           cfg.ABI,
		ramSizeMB:     cfg.RAMSize,
		heapSizeMB:    cfg.HeapSize,
		autoProvision: cfg.AutoProvision,
		avdTagCache:   make(map[string]tagInfo),
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
		info := p.tagFor(name)
		p.portMu.Lock()
		port, ok := p.avdPortMap[name]
		p.portMu.Unlock()
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
			ATD:      info.atd,
			EstRAMMB: info.estRAMMB,
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

	tag, tagErr := p.resolveTargetTag(ctx, target)
	isATD := tagErr == nil && strings.Contains(tag, "atd")
	if isATD {
		ram := p.ramSizeMB
		if ram == 0 {
			ram = 1536
		}
		args = append(args, atdFlags(ram)...)
	} else if opts.NoWindow {
		args = append(args, "-no-window")
	}

	// Detach the emulator process from the request context: the emulator must
	// outlive the boot_device call that spawned it (exec.CommandContext kills
	// the child when the ctx is cancelled, which would tear the emulator down
	// as soon as the HTTP response is flushed).
	spawnCtx := context.WithoutCancel(ctx)
	cmd := p.emulatorCmd(spawnCtx, args...)
	setProcAttr(cmd)
	if err := cmd.Start(); err != nil {
		return contract.Device{}, fmt.Errorf("emulator start: %w", err)
	}

	p.portMu.Lock()
	p.avdPortMap[target] = port
	p.portMu.Unlock()
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
	p.portMu.Lock()
	port, ok := p.avdPortMap[target]
	p.portMu.Unlock()
	if !ok {
		return fmt.Errorf("no port mapping for AVD: %s", target)
	}
	serial := fmt.Sprintf("emulator-%d", port)
	_ = p.adbCmd(ctx, "-s", serial, "emu", "kill").Run()
	return nil
}

// State returns the state of an emulator.
func (p *Platform) State(ctx context.Context, target string) (contract.DeviceState, error) {
	p.portMu.Lock()
	port, ok := p.avdPortMap[target]
	p.portMu.Unlock()
	if !ok {
		// Try to discover port from adb devices.
		out, err := p.adbCmd(ctx, "devices").Output()
		if err != nil {
			// adb failure (e.g. server not running yet) means we cannot see the
			// device; report stopped instead of erroring so Boot can proceed to
			// Start (which spawns the emulator and warms adb itself).
			return contract.DeviceStateStopped, nil
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
					p.portMu.Lock()
					p.avdPortMap[target] = n
					p.portMu.Unlock()
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
		// The serial is not registered with adb (cold server, device gone):
		// treat as stopped rather than a hard error, mirroring the discovery
		// branch above.
		return contract.DeviceStateStopped, nil
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
	// Restart with -wipe-data; detached so it survives the request.
	cmd := p.emulatorCmd(context.WithoutCancel(ctx), "-avd", target, "-wipe-data")
	setProcAttr(cmd)
	_ = cmd.Start()
	return nil
}

// OpenURL opens a deep link on the emulator.
func (p *Platform) OpenURL(ctx context.Context, target, url string) error {
	p.portMu.Lock()
	port, ok := p.avdPortMap[target]
	p.portMu.Unlock()
	if !ok {
		return fmt.Errorf("no port mapping for AVD: %s", target)
	}
	serial := fmt.Sprintf("emulator-%d", port)
	cmd := p.adbCmd(ctx, "-s", serial, "shell", "am", "start", "-a", "android.intent.action.VIEW", "-d", url)
	return cmd.Run()
}

// Capabilities reports the supported operations: base lifecycle caps only —
// the Android adapter does not implement Optimizer.
func (p *Platform) Capabilities() contract.CapabilitySet {
	return contract.CapAll
}
