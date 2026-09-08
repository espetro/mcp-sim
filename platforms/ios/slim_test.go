package ios

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/espetro/mcp-sim/internal/config"
	"github.com/espetro/mcp-sim/pkg/contract"
)

// fakeSimslimDir writes a shell script named `simslim` into a temp dir that
// records invocations and emits canned JSON, then returns the dir for PATH.
func fakeSimslimDir(t *testing.T, statusJSON, measureJSON string) string {
	t.Helper()
	dir := t.TempDir()
	script := `#!/bin/sh
echo "$@" >> "` + filepath.Join(dir, "calls.log") + `"
cmd="$1"; shift
case "$cmd" in
  version) echo "simslim 0.8.0" ;;
  status) cat <<'EOF'
` + statusJSON + `
EOF
  ;;
  measure) cat <<'EOF'
` + measureJSON + `
EOF
  ;;
  on|off) exit 0 ;;
  *) exit 0 ;;
esac
`
	path := filepath.Join(dir, "simslim")
	// #nosec G306 -- executable script required for PATH-based fixture
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return dir
}

func callsLog(t *testing.T, dir string) []string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, "calls.log"))
	if err != nil {
		return nil
	}
	s := strings.TrimSpace(string(data))
	if s == "" {
		return nil
	}
	return strings.Split(s, "\n")
}

func withPATH(t *testing.T, dir string) {
	t.Helper()
	old := os.Getenv("PATH")
	// #nosec G104 -- PATH mutation is safe in tests and restored by cleanup
	_ = os.Setenv("PATH", dir+string(os.PathListSeparator)+old) //nolint:errcheck
	t.Cleanup(func() { _ = os.Setenv("PATH", old) })            //nolint:errcheck
}

const slimStatus = `{"managedDisabled":42,"managedTotal":42,"booted":true,"persistent":true,"verdict":"slim"}`
const slimMeasure = `{"processes":31,"bytes":515000000,"cpu":2.5}`

func TestNewSlimmerDisabled(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("PATH probing test only meaningful with exec; works everywhere but keep fast")
	}
	dir := fakeSimslimDir(t, slimStatus, slimMeasure)
	withPATH(t, dir)
	if s := NewSlimmer(config.SlimConfig{Enabled: false}); s != nil {
		t.Fatal("expected nil slimmer when disabled")
	}
}

func TestNewSlimmerEnabled(t *testing.T) {
	dir := fakeSimslimDir(t, slimStatus, slimMeasure)
	withPATH(t, dir)
	if s := NewSlimmer(config.SlimConfig{Enabled: true}); s == nil {
		t.Fatal("expected slimmer when simslim on PATH")
	}
}

func TestNewSlimmerTooOld(t *testing.T) {
	dir := t.TempDir()
	script := "#!/bin/sh\necho 'simslim 0.5.1'\n"
	// #nosec G306 -- executable script required for PATH-based fixture
	if err := os.WriteFile(filepath.Join(dir, "simslim"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	withPATH(t, dir)
	if s := NewSlimmer(config.SlimConfig{Enabled: true}); s != nil {
		t.Fatal("expected nil slimmer for version < 0.6.0")
	}
}

func TestOptimizeStatusParsesJSON(t *testing.T) {
	dir := fakeSimslimDir(t, slimStatus, slimMeasure)
	withPATH(t, dir)
	s := NewSlimmer(config.SlimConfig{Enabled: true})
	if s == nil {
		t.Fatal("slimmer nil")
	}
	st, err := s.OptimizeStatus(context.Background(), "UDID")
	if err != nil {
		t.Fatal(err)
	}
	if !st.Slimmed || !st.Persistent || st.ManagedDisabled != 42 || st.ManagedTotal != 42 {
		t.Fatalf("unexpected status: %+v", st)
	}
}

func TestMeasureParsesJSON(t *testing.T) {
	dir := fakeSimslimDir(t, slimStatus, slimMeasure)
	withPATH(t, dir)
	s := NewSlimmer(config.SlimConfig{Enabled: true})
	m, err := s.Measure(context.Background(), "UDID")
	if err != nil {
		t.Fatal(err)
	}
	if m.PhysFootprintBytes != 515000000 || m.ProcessCount != 31 || m.CPUPercent != 2.5 {
		t.Fatalf("unexpected usage: %+v", m)
	}
}

func TestOptimizeArgs(t *testing.T) {
	dir := fakeSimslimDir(t, slimStatus, slimMeasure)
	withPATH(t, dir)
	s := NewSlimmer(config.SlimConfig{
		Enabled:      true,
		BootTimeout:  "15m",
		SpawnTimeout: "3m",
	})
	ctx := context.Background()

	if err := s.Optimize(ctx, "UDID", contract.OptimizeOpts{
		Except:   []string{"push", "store"},
		Keep:     []string{"com.apple.apsd"},
		NoReboot: true,
	}); err != nil {
		t.Fatal(err)
	}
	calls := callsLog(t, dir)
	want := "on UDID --except push,store --keep com.apple.apsd --no-reboot"
	if len(calls) == 0 || calls[len(calls)-1] != want {
		t.Fatalf("got %q, want %q", calls, want)
	}

	if err := s.Restore(ctx, "UDID"); err != nil {
		t.Fatal(err)
	}
	calls = callsLog(t, dir)
	if calls[len(calls)-1] != "off UDID" {
		t.Fatalf("got %q", calls[len(calls)-1])
	}
}

func TestOptimizeProfileArg(t *testing.T) {
	dir := fakeSimslimDir(t, slimStatus, slimMeasure)
	withPATH(t, dir)
	s := NewSlimmer(config.SlimConfig{Enabled: true})
	if err := s.Optimize(context.Background(), "UDID", contract.OptimizeOpts{Profile: "/tmp/ci.json"}); err != nil {
		t.Fatal(err)
	}
	calls := callsLog(t, dir)
	if calls[len(calls)-1] != "on UDID --profile /tmp/ci.json" {
		t.Fatalf("got %q", calls[len(calls)-1])
	}
}

func TestOptimizeTimeoutEnvPassthrough(t *testing.T) {
	dir := fakeSimslimDir(t, slimStatus, slimMeasure)
	withPATH(t, dir)
	s := NewSlimmer(config.SlimConfig{Enabled: true, BootTimeout: "15m", SpawnTimeout: "3m"})
	cmd := s.command(context.Background(), "on", "UDID")
	joined := strings.Join(cmd.Env, " ")
	if !strings.Contains(joined, "SIMSLIM_BOOT_TIMEOUT=15m") || !strings.Contains(joined, "SIMSLIM_SPAWN_TIMEOUT=3m") {
		t.Fatalf("timeout env missing: %v", cmd.Env)
	}
	if len(cmd.Env) > 2 {
		t.Fatalf("env should not be global, got %v", cmd.Env)
	}
}

func TestSupportsPersistentOverrides(t *testing.T) {
	cases := []struct {
		runtime string
		want    bool
	}{
		{"iOS 18.5", true},
		{"iOS 18.4", false},
		{"iOS 19.0", true},
		{"iOS 17.5", false},
		{"iOS-18-5", true},
		{"iOS-16-4", false},
		{"", true}, // unknown: assume yes, simslim errors precisely
	}
	for _, c := range cases {
		if got := supportsPersistentOverrides(c.runtime); got != c.want {
			t.Errorf("supportsPersistentOverrides(%q) = %v, want %v", c.runtime, got, c.want)
		}
	}
}

func TestVersionLess(t *testing.T) {
	if !versionLess("0.5.9", "0.6.0") {
		t.Error("0.5.9 < 0.6.0")
	}
	if versionLess("0.8.0", "0.6.0") {
		t.Error("0.8.0 not < 0.6.0")
	}
	if versionLess("1.0", "0.9.9") {
		t.Error("1.0 not < 0.9.9")
	}
}

func TestShouldOptimizeOnBoot(t *testing.T) {
	dir := fakeSimslimDir(t, slimStatus, slimMeasure)
	withPATH(t, dir)
	p := &Platform{slim: NewSlimmer(config.SlimConfig{Enabled: true, OnBoot: true})}

	no := false
	yes := true
	if !p.ShouldOptimizeOnBoot(nil) {
		t.Error("nil override should use config default (true)")
	}
	if p.ShouldOptimizeOnBoot(&no) {
		t.Error("explicit false should win")
	}
	if !p.ShouldOptimizeOnBoot(&yes) {
		t.Error("explicit true should win")
	}

	p2 := &Platform{slim: NewSlimmer(config.SlimConfig{Enabled: true, OnBoot: false})}
	if p2.ShouldOptimizeOnBoot(nil) {
		t.Error("config default false")
	}
	if !p2.ShouldOptimizeOnBoot(&yes) {
		t.Error("per-call true should override config false")
	}
}

// JSON round-trip sanity for status parsing of stock devices.
func TestStatusJSONStockDevice(t *testing.T) {
	dir := fakeSimslimDir(t, `{"managedDisabled":0,"managedTotal":42,"booted":false,"persistent":true,"verdict":"stock"}`, slimMeasure)
	withPATH(t, dir)
	s := NewSlimmer(config.SlimConfig{Enabled: true})
	st, err := s.OptimizeStatus(context.Background(), "UDID")
	if err != nil {
		t.Fatal(err)
	}
	if st.Slimmed {
		t.Fatalf("stock device should not be slimmed: %+v", st)
	}
	var raw map[string]any
	if err := json.Unmarshal([]byte(slimStatus), &raw); err != nil {
		t.Fatal(err)
	}
}
