//go:build linux

// Package redroid implements contract.Platform for Redroid (Remote Android
// in Docker) containers on Linux hosts.
//
// WIP: this adapter is UNTESTED on real hardware. It was authored on macOS,
// where Redroid cannot run (Docker VMs lack binder kernel modules; see
// docs/redroid.md). It is validated only by compilation under GOOS=linux and
// by pure logic unit tests (name derivation, docker ps parsing, port
// selection). Exercise it on a Linux host before relying on it.
package redroid

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/espetro/mcp-sim/pkg/contract"
)

const (
	// defaultPrefix namespaces containers created by this adapter so List
	// can filter with docker ps --filter name=<prefix>-.
	defaultPrefix = "mcp-sim-redroid"
	// defaultImage is the Redroid image used unless REDROID_IMAGE overrides
	// it. 12.0.0 64only is the common baseline for modern arm64/x86_64 hosts.
	defaultImage = "redroid/redroid:12.0.0-latest_64only"
	// basePort is the first host port probed when mapping a container's
	// adb (5555) listener.
	basePort = 5555
	// internalPort is the port redroid listens on inside the container.
	internalPort = 5555
)

// Platform manages Redroid containers via docker and inspects them via adb.
type Platform struct {
	dockerBin       string
	adbBin          string
	containerPrefix string
	image           string

	mu sync.Mutex
	// targets maps target ID (== container name suffix) to its adb host port.
	targets map[string]int
}

// New creates a new Redroid platform adapter.
//
// Resilient to missing tooling: returns (nil, nil) when docker is not in
// PATH. Callers should treat a nil result as "skip redroid registration"
// rather than a fatal error. The host must be Linux; on other GOOS this
// package does not compile.
func New() (*Platform, error) {
	dockerBin, err := exec.LookPath("docker")
	if err != nil {
		return nil, nil
	}
	adbBin, _ := exec.LookPath("adb") // optional; only needed for boot checks and OpenURL
	image := os.Getenv("REDROID_IMAGE")
	if image == "" {
		image = defaultImage
	}
	prefix := os.Getenv("MCPSIM_REDROID_PREFIX")
	if prefix == "" {
		prefix = defaultPrefix
	}
	return &Platform{
		dockerBin:       dockerBin,
		adbBin:          adbBin,
		containerPrefix: prefix,
		image:           image,
		targets:         make(map[string]int),
	}, nil
}

// Name returns the platform identifier.
func (p *Platform) Name() string { return "redroid" }

// containerName derives the docker container name for a target. Exported for
// tests within the package only (lowercase would be unexported, but keeping
// it a plain function makes it directly unit testable).
func (p *Platform) containerName(target string) string {
	return p.containerPrefix + "-" + target
}

// Start boots a Redroid container for the target and records its adb port.
//
// Wipe semantics note: because each container starts from a fresh image
// layer, Start of a previously stopped target yields fresh user data. This
// adapter does not preserve data across Stop/Start cycles.
func (p *Platform) Start(ctx context.Context, target string, opts contract.StartOpts) (contract.Device, error) {
	name := p.containerName(target)

	port := opts.Port
	if port == 0 {
		var err error
		port, err = pickFreePort()
		if err != nil {
			return contract.Device{}, fmt.Errorf("redroid: picking free port: %w", err)
		}
	}

	args := []string{
		"run", "-d", "--privileged",
		"--name", name,
		"-p", fmt.Sprintf("%d:%d", port, internalPort),
		p.image,
		"--memory-swappiness=0",
	}
	if err := p.docker(ctx, args...); err != nil {
		return contract.Device{}, fmt.Errorf("redroid: starting container %s: %w", name, err)
	}

	p.mu.Lock()
	p.targets[target] = port
	p.mu.Unlock()

	return contract.Device{
		ID:       target,
		Name:     name,
		Platform: p.Name(),
		State:    contract.DeviceStateBooting,
	}, nil
}

// Stop force removes the container (docker rm -f). Data is not preserved.
func (p *Platform) Stop(ctx context.Context, target string) error {
	if err := p.docker(ctx, "rm", "-f", p.containerName(target)); err != nil {
		return fmt.Errorf("redroid: removing container: %w", err)
	}
	p.mu.Lock()
	delete(p.targets, target)
	p.mu.Unlock()
	return nil
}

// State reports the device state from container status plus adb boot state.
func (p *Platform) State(ctx context.Context, target string) (contract.DeviceState, error) {
	status, err := p.containerStatus(ctx, target)
	if err != nil {
		return contract.DeviceStateUnknown, err
	}
	state := statusToDeviceState(status)
	if state == contract.DeviceStateRunning {
		// Container running does not imply Android booted; refine via adb.
		p.mu.Lock()
		port, ok := p.targets[target]
		p.mu.Unlock()
		if ok && p.adbBin != "" && p.bootCompleted(ctx, port) {
			return contract.DeviceStateRunning, nil
		}
		return contract.DeviceStateBooting, nil
	}
	return state, nil
}

// AwaitReady polls adb connect and sys.boot_completed until booted or timeout.
func (p *Platform) AwaitReady(ctx context.Context, target string, timeout time.Duration) error {
	if p.adbBin == "" {
		return fmt.Errorf("redroid: adb not found in PATH, cannot await readiness")
	}
	p.mu.Lock()
	port, ok := p.targets[target]
	p.mu.Unlock()
	if !ok {
		return fmt.Errorf("redroid: target %q not started by this process", target)
	}
	serial := fmt.Sprintf("localhost:%d", port)

	deadline := time.Now().Add(timeout)
	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		// Best effort connect; errors are expected until the container's
		// adbd is listening.
		_ = p.adb(ctx, "connect", serial)
		if p.bootCompleted(ctx, port) {
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("redroid: timeout waiting for %s to boot", serial)
		}
		time.Sleep(2 * time.Second)
	}
}

// List enumerates containers belonging to this adapter via docker ps -a.
func (p *Platform) List(ctx context.Context) ([]contract.Device, error) {
	out, err := p.output(ctx, "ps", "-a",
		"--filter", "name="+p.containerPrefix+"-",
		"--format", "{{.Names}}\t{{.Status}}")
	if err != nil {
		return nil, fmt.Errorf("redroid: listing containers: %w", err)
	}
	devices := []contract.Device{}
	for _, line := range strings.Split(out, "\n") {
		name, status, ok := parseDockerPSLine(line)
		if !ok {
			continue
		}
		target := strings.TrimPrefix(name, p.containerPrefix+"-")
		devices = append(devices, contract.Device{
			ID:       target,
			Name:     name,
			Platform: p.Name(),
			State:    statusToDeviceState(status),
		})
	}
	return devices, nil
}

// Wipe force removes the container. A subsequent Start boots a brand new
// container from the image, which is by definition fresh user data.
func (p *Platform) Wipe(ctx context.Context, target string) error {
	return p.Stop(ctx, target)
}

// OpenURL launches a deep link on the device via adb am start.
func (p *Platform) OpenURL(ctx context.Context, target, url string) error {
	p.mu.Lock()
	port, ok := p.targets[target]
	p.mu.Unlock()
	if !ok {
		return fmt.Errorf("redroid: target %q not started by this process", target)
	}
	if err := p.adb(ctx, "-s", fmt.Sprintf("localhost:%d", port),
		"shell", "am", "start", "-a", "android.intent.action.VIEW", "-d", url); err != nil {
		return fmt.Errorf("redroid: opening URL: %w", err)
	}
	return nil
}

// Capabilities reports the base lifecycle set. Redroid has no Optimizer, so
// CapOptimize/CapMeasure are excluded per the pre-1.0 contract.
func (p *Platform) Capabilities() contract.CapabilitySet {
	return contract.CapAll
}

// docker runs a docker command, discarding stdout.
func (p *Platform) docker(ctx context.Context, args ...string) error {
	_, err := p.output(ctx, args...)
	return err
}

// output runs a docker command and returns trimmed stdout.
func (p *Platform) output(ctx context.Context, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, p.dockerBin, args...)
	out, err := cmd.CombinedOutput()
	return strings.TrimSpace(string(out)), err
}

// adb runs an adb command, discarding output.
func (p *Platform) adb(ctx context.Context, args ...string) error {
	cmd := exec.CommandContext(ctx, p.adbBin, args...)
	_, err := cmd.CombinedOutput()
	return err
}

// containerStatus returns the docker Status column for the target's
// container, or an error when the container does not exist.
func (p *Platform) containerStatus(ctx context.Context, target string) (string, error) {
	name := p.containerName(target)
	out, err := p.output(ctx, "ps", "-a",
		"--filter", "name=^/"+name+"$",
		"--format", "{{.Status}}")
	if err != nil {
		return "", err
	}
	if out == "" {
		return "", fmt.Errorf("redroid: container %s not found", name)
	}
	return out, nil
}

// bootCompleted reports whether adb reports sys.boot_completed == 1.
func (p *Platform) bootCompleted(ctx context.Context, port int) bool {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, p.adbBin, "-s", fmt.Sprintf("localhost:%d", port),
		"shell", "getprop", "sys.boot_completed")
	out, err := cmd.Output()
	if err != nil {
		return false
	}
	return strings.TrimSpace(string(out)) == "1"
}

// parseDockerPSLine splits a docker ps --format line into container name and
// status. It is tolerant of status strings containing tabs.
func parseDockerPSLine(line string) (name, status string, ok bool) {
	line = strings.TrimSpace(line)
	name, status, found := strings.Cut(line, "\t")
	if !found || name == "" {
		return "", "", false
	}
	return name, strings.TrimSpace(status), true
}

// statusToDeviceState maps a docker ps Status string to a DeviceState.
func statusToDeviceState(status string) contract.DeviceState {
	s := strings.ToLower(strings.TrimSpace(status))
	switch {
	case s == "":
		return contract.DeviceStateUnknown
	case strings.HasPrefix(s, "up"):
		return contract.DeviceStateRunning
	case strings.HasPrefix(s, "exited"), strings.HasPrefix(s, "created"), strings.HasPrefix(s, "dead"):
		return contract.DeviceStateStopped
	case strings.HasPrefix(s, "restarting"):
		return contract.DeviceStateBooting
	default:
		return contract.DeviceStateUnknown
	}
}

// pickFreePort returns a free TCP port >= 5555 by binding to :0 and, if the
// ephemeral port is below the base, probing upward from the base.
func pickFreePort() (int, error) {
	for port := basePort; port < basePort+1024; port++ {
		ln, err := net.Listen("tcp", net.JoinHostPort("localhost", strconv.Itoa(port)))
		if err != nil {
			continue // occupied, try next
		}
		_ = ln.Close()
		return port, nil
	}
	// Fall back to an ephemeral port; it satisfies "free" though not >= 5555.
	ln, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		return 0, err
	}
	defer ln.Close()
	return ln.Addr().(*net.TCPAddr).Port, nil
}
