package ios

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"strconv"
	"strings"

	"github.com/espetro/mcp-sim/internal/config"
	"github.com/espetro/mcp-sim/pkg/contract"
)

// slimmer implements contract.Optimizer by shelling out to the simslim CLI
// (https://github.com/mobai-app/simslim). simslim is macOS-only, but this
// wrapper only execs it, so it builds everywhere; the binary is probed at
// registration time (see NewSlimmer).
//
// simslim has no --help flag; do not probe with it.
type slimmer struct {
	cfg config.SlimConfig
}

// simslimVersionMin is the lowest CLI version this wrapper supports
// (stable --json output for status/measure).
const simslimVersionMin = "0.6.0"

// NewSlimmer returns a simslim-backed optimizer, or nil when simslim is not
// usable (absent, too old, or disabled). Callers treat nil as "skip slim".
func NewSlimmer(cfg config.SlimConfig) *slimmer {
	if !cfg.Enabled {
		return nil
	}
	path, err := exec.LookPath("simslim")
	if err != nil {
		return nil
	}
	if v, err := simslimVersion(path); err != nil || versionLess(v, simslimVersionMin) {
		return nil
	}
	return &slimmer{cfg: cfg}
}

// simslimVersion runs `simslim version` and returns e.g. "0.8.0".
func simslimVersion(path string) (string, error) {
	out, err := exec.CommandContext(context.Background(), path, "version").Output() // #nosec G114 -- version probe is fast
	if err != nil {
		return "", err
	}
	fields := strings.Fields(string(bytes.TrimSpace(out)))
	if len(fields) < 2 { // "simslim 0.8.0"
		return "", fmt.Errorf("unexpected simslim version output")
	}
	v := strings.TrimPrefix(fields[1], "v")
	if _, err := strconv.Atoi(strings.SplitN(v, ".", 2)[0]); err != nil {
		return "", fmt.Errorf("parsing simslim version %q: %w", v, err)
	}
	return v, nil
}

// versionLess compares dotted numeric versions a < b.
func versionLess(a, b string) bool {
	as := strings.Split(a, ".")
	bs := strings.Split(b, ".")
	for i := range max(len(as), len(bs)) {
		var ai, bi int
		if i < len(as) {
			ai, _ = strconv.Atoi(as[i])
		}
		if i < len(bs) {
			bi, _ = strconv.Atoi(bs[i])
		}
		if ai != bi {
			return ai < bi
		}
	}
	return false
}

func (s *slimmer) command(ctx context.Context, args ...string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, "simslim", args...)
	// Timeout passthrough, set on this command only (never global mutation).
	if s.cfg.BootTimeout != "" {
		cmd.Env = append(cmd.Env, "SIMSLIM_BOOT_TIMEOUT="+s.cfg.BootTimeout)
	}
	if s.cfg.SpawnTimeout != "" {
		cmd.Env = append(cmd.Env, "SIMSLIM_SPAWN_TIMEOUT="+s.cfg.SpawnTimeout)
	}
	return cmd
}

func (s *slimmer) run(ctx context.Context, args ...string) (string, error) {
	cmd := s.command(ctx, args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return "", fmt.Errorf("simslim %s: %s", args[0], msg)
	}
	return string(out), nil
}

// Optimize slims the device via `simslim on`. It mutates o (setting NoReboot
// when the runtime cannot persist overrides) so callers can surface the
// non-persistence warning.
func (s *slimmer) Optimize(ctx context.Context, target string, o contract.OptimizeOpts) error {
	args := []string{"on", target}
	if o.Profile != "" {
		args = append(args, "--profile", o.Profile)
	} else {
		if len(o.Except) > 0 {
			args = append(args, "--except", strings.Join(o.Except, ","))
		}
		if len(o.Keep) > 0 {
			args = append(args, "--keep", strings.Join(o.Keep, ","))
		}
	}
	if o.NoReboot {
		args = append(args, "--no-reboot")
	}
	_, err := s.run(ctx, args...)
	return err
}

// Restore returns the device to stock via `simslim off`.
func (s *slimmer) Restore(ctx context.Context, target string) error {
	_, err := s.run(ctx, "off", target)
	return err
}

// simslimStatus matches `simslim status <udid> --json` output.
type simslimStatus struct {
	ManagedDisabled int  `json:"managedDisabled"`
	ManagedTotal    int  `json:"managedTotal"`
	Booted          bool `json:"booted"`
	Persistent      bool `json:"persistent"`
	Verdict         string `json:"verdict"`
}

// OptimizeStatus reports how slim the device currently is.
func (s *slimmer) OptimizeStatus(ctx context.Context, target string) (contract.OptimizeStatus, error) {
	out, err := s.run(ctx, "status", target, "--json")
	if err != nil {
		return contract.OptimizeStatus{}, err
	}
	var st simslimStatus
	if err := json.Unmarshal([]byte(out), &st); err != nil {
		return contract.OptimizeStatus{}, fmt.Errorf("parse simslim status json: %w", err)
	}
	return contract.OptimizeStatus{
		Slimmed:         st.ManagedDisabled > 0,
		Persistent:      st.Persistent,
		ManagedDisabled: st.ManagedDisabled,
		ManagedTotal:    st.ManagedTotal,
	}, nil
}

// simslimMeasurement matches `simslim measure <udid> --json` output.
type simslimMeasurement struct {
	Processes int     `json:"processes"`
	Bytes     int64   `json:"bytes"`
	CPU       float64 `json:"cpu"`
}

// Measure returns the device's summed phys_footprint and process count.
func (s *slimmer) Measure(ctx context.Context, target string) (contract.ResourceUsage, error) {
	out, err := s.run(ctx, "measure", target, "--json")
	if err != nil {
		return contract.ResourceUsage{}, err
	}
	var m simslimMeasurement
	if err := json.Unmarshal([]byte(out), &m); err != nil {
		return contract.ResourceUsage{}, fmt.Errorf("parse simslim measure json: %w", err)
	}
	return contract.ResourceUsage{
		PhysFootprintBytes: m.Bytes,
		ProcessCount:       m.Processes,
		CPUPercent:         m.CPU,
	}, nil
}

// ShouldOptimizeOnBoot resolves the per-call Optimize override against the
// configured on_boot default. Nil pointer = config default.
func (p *Platform) ShouldOptimizeOnBoot(override *bool) bool {
	if override != nil {
		return *override
	}
	return p.slim != nil && p.slim.cfg.OnBoot
}

// supportsPersistentOverrides reports whether the device's runtime can keep
// launchd disable overrides across reboots (iOS >= 18.5).
func supportsPersistentOverrides(runtime string) bool {
	// Runtime strings look like "iOS 18.5" or map keys like "iOS-18-5".
	norm := strings.Map(func(r rune) rune {
		if r == '-' {
			return '.'
		}
		return r
	}, runtime)
	fields := strings.Fields(norm)
	for _, f := range fields {
		f = strings.TrimPrefix(strings.TrimPrefix(f, "iOS"), ".")
		if f == "" {
			continue
		}
		parts := strings.SplitN(f, ".", 3)
		if len(parts) < 2 {
			continue
		}
		major, err1 := strconv.Atoi(parts[0])
		minor, err2 := strconv.Atoi(parts[1])
		if err1 != nil || err2 != nil {
			continue
		}
		return major > 18 || (major == 18 && minor >= 5)
	}
	// Unknown runtime: assume yes; simslim on will surface a precise error
	// and OptimizeOnBoot falls back to --no-reboot on failure.
	return true
}

// runtimeVersion returns the device's runtime string from List (Version field).
func (p *Platform) runtimeVersion(ctx context.Context, target string) string {
	devs, err := p.List(ctx)
	if err != nil {
		return ""
	}
	for _, d := range devs {
		if d.ID == target {
			return d.Version
		}
	}
	return ""
}

// OptimizeOnBoot slims a freshly booted device per config. On runtimes that
// cannot persist overrides (iOS < 18.5) it retries with --no-reboot, which
// slims the current boot session only; callers surface the non-persistent
// warning via OptimizeStatus.
func (p *Platform) OptimizeOnBoot(ctx context.Context, target string, override *bool) error {
	if p.slim == nil || !p.ShouldOptimizeOnBoot(override) {
		return nil
	}
	opts := contract.OptimizeOpts{
		Profile: p.slim.cfg.Profile,
		Except:  p.slim.cfg.Except,
		Keep:    p.slim.cfg.Keep,
	}
	if !supportsPersistentOverrides(p.runtimeVersion(ctx, target)) {
		opts.NoReboot = true
	}
	err := p.slim.Optimize(ctx, target, opts)
	if err != nil && !opts.NoReboot {
		// simslim on errors outright on iOS < 18.5 without --no-reboot
		// ("runtime cannot persist launchd disable overrides"); retry in
		// session-only mode rather than failing the boot flow.
		opts.NoReboot = true
		err = p.slim.Optimize(ctx, target, opts)
	}
	return err
}

// ReSlimAfterWipe re-applies slimming after a successful erase: erase resets
// launchd overrides to stock (documented simslim behavior), so devices under
// slim.on_boot get re-slimmed automatically. No-op when slim is off.
func (p *Platform) ReSlimAfterWipe(ctx context.Context, target string) error {
	if p.slim == nil {
		return nil
	}
	st, err := p.slim.OptimizeStatus(ctx, target)
	if err != nil {
		// Status needs simctl; on a just-erased device this can race.
		// Optimistically re-apply per config instead of failing the wipe.
		st = contract.OptimizeStatus{Slimmed: false}
	}
	if !st.Slimmed && p.slim.cfg.OnBoot {
		return p.OptimizeOnBoot(ctx, target, nil)
	}
	return nil
}

var (
	_ contract.Optimizer = (*slimmer)(nil)
	_ io.Writer          = (*bytes.Buffer)(nil) // keep io import if unused paths change
)
