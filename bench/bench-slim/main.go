// Command bench-slim runs the mcp-sim x simslim benchmark matrix:
//
//  1. memory: stock vs slim phys_footprint per booted simulator
//  2. latency: boot+await_ready time, on_boot-slim vs stock
//  3. wipe-reslim: wipe -> status shows slimmed again
//  4. seam: open_url + get_state success rate across iterations
//
// Usage:
//
//	go run ./bench/bench-slim -udids <udid1>,<udid2> -iters 5 -out bench/results
//
// Results are written as JSON + Markdown. Machine specs are captured in the
// header. Requires: macOS, Xcode simctl, simslim >= 0.6.0.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

type result struct {
	Timestamp string          `json:"timestamp"`
	Machine   machineSpecs    `json:"machine"`
	Memory    []memSample     `json:"memory"`
	Latency   latencyRes      `json:"latency"`
	WipeRes   wipeRes         `json:"wipe_reslim"`
	Seam      seamRes         `json:"seam"`
	MCPvsCLI  *workflowResult `json:"mcp_vs_cli,omitempty"`
	MCPShape  string          `json:"mcp_contract_shape,omitempty"`
}

type machineSpecs struct {
	Model    string `json:"model"`
	Chip     string `json:"chip"`
	MemoryGB int    `json:"memory_gb"`
	MacOS    string `json:"macos"`
	Simslim  string `json:"simslim"`
	Runtime  string `json:"ios_runtime"`
}

type memSample struct {
	Label     string `json:"label"` // "stock" | "slim"
	UDID      string `json:"udid"`
	Bytes     int64  `json:"phys_footprint_bytes"`
	Processes int    `json:"process_count"`
	SwapBytes int64  `json:"swap_used_bytes"`
}

type latencyRes struct {
	StockBootMS []int64 `json:"stock_boot_ms"`
	SlimBootMS  []int64 `json:"slim_boot_ms"` // boot with on_boot slim applied
}

type wipeRes struct {
	Runs         int  `json:"runs"`
	AllReslimmed bool `json:"all_reslimmed"`
}

type seamRes struct {
	Iterations   int `json:"iterations"`
	StockOK      int `json:"stock_ok"`
	StockOpenURL int `json:"stock_openurl_ok"`
	SlimOK       int `json:"slim_ok"`
	SlimOpenURL  int `json:"slim_openurl_ok"`
}

func main() {
	udids := flag.String("udids", "", "comma-separated simulator UDIDs to use (default: first 2 iPhone simulators)")
	iters := flag.Int("iters", 5, "iterations per latency/seam measurement")
	outDir := flag.String("out", "bench/results", "output directory")
	flag.Parse()

	ctx := context.Background()
	runOrchestratorSmoke(ctx)
	res := result{Timestamp: time.Now().UTC().Format(time.RFC3339)}
	res.Machine = collectSpecs(ctx)

	targets := strings.Split(*udids, ",")
	if *udids == "" || len(targets) == 0 || targets[0] == "" {
		var err error
		targets, err = firstIPhones(3)
		if err != nil {
			fatal(err)
		}
	}
	fmt.Printf("bench targets: %v\n", targets)

	shutdownAll(targets)

	// --- 1. memory: boot all stock, measure; slim, measure.
	// simslim state persists across reboots, so force stock first.
	for _, u := range targets {
		must(bootAndAwait(ctx, u))
		must(slimOff(ctx, u))
	}
	for _, u := range targets {
		must(bootAndAwait(ctx, u))
	}
	time.Sleep(10 * time.Second) // settle
	for _, u := range targets {
		m, err := measure(ctx, u)
		if err != nil {
			fatal(fmt.Errorf("measure stock %s: %w", u, err))
		}
		m.Label, m.UDID = "stock", u
		m.SwapBytes = swapUsed()
		res.Memory = append(res.Memory, m)
		fmt.Printf("stock  %s: %d bytes, %d procs\n", short(u), m.Bytes, m.Processes)
	}

	for _, u := range targets {
		must(slimOn(ctx, u))
	}
	time.Sleep(10 * time.Second) // settle after reboot
	for _, u := range targets {
		m, err := measure(ctx, u)
		if err != nil {
			fatal(fmt.Errorf("measure slim %s: %w", u, err))
		}
		m.Label, m.UDID = "slim", u
		m.SwapBytes = swapUsed()
		res.Memory = append(res.Memory, m)
		fmt.Printf("slim   %s: %d bytes, %d procs\n", short(u), m.Bytes, m.Processes)
	}

	// --- 2. latency: N boots stock vs N boots with slim applied post-boot-on.
	shutdownAll(targets)
	u := targets[0]
	for i := 0; i < *iters; i++ {
		start := time.Now()
		must(bootAndAwait(ctx, u))
		res.Latency.StockBootMS = append(res.Latency.StockBootMS, time.Since(start).Milliseconds())
		must(shutdown(ctx, u))
	}
	for i := 0; i < *iters; i++ {
		start := time.Now()
		must(bootAndAwait(ctx, u))
		must(slimOn(ctx, u)) // includes slim reboot cycle where supported
		must(bootAndAwait(ctx, u))
		res.Latency.SlimBootMS = append(res.Latency.SlimBootMS, time.Since(start).Milliseconds())
		must(slimOff(ctx, u))
		must(shutdown(ctx, u))
	}
	fmt.Printf("latency: stock %v ms, slim %v ms\n", res.Latency.StockBootMS, res.Latency.SlimBootMS)

	// --- 3. wipe/re-slim correctness (mcp-sim handles this in wipe_device;
	// here we verify the primitive: erase resets overrides, slim re-applies).
	for _, u := range targets {
		// simslim on/off need a booted simulator.
		must(bootAndAwait(ctx, u))
		must(slimOn(ctx, u))
		must(erase(ctx, u))
		must(bootAndAwait(ctx, u))
		st, err := status(ctx, u)
		if err != nil {
			fatal(err)
		}
		if st.Slimmed() {
			res.WipeRes.Runs++
			continue // unexpectedly slim after erase; simslim semantics say stock
		}
		must(slimOn(ctx, u))
		st, err = status(ctx, u)
		if err != nil {
			fatal(err)
		}
		if st.Slimmed() {
			res.WipeRes.AllReslimmed = true
		}
		must(slimOff(ctx, u))
		must(shutdown(ctx, u))
	}

	// --- 4. seam precision: open_url + get_state-equivalent on stock vs slim.
	deepLink := "https://www.apple.com"
	for i := 0; i < *iters; i++ {
		for _, u := range targets {
			must(bootAndAwait(ctx, u))
			if err := openURL(ctx, u, deepLink); err == nil {
				res.Seam.StockOpenURL++
			}
			if _, err := status(ctx, u); err == nil {
				res.Seam.StockOK++
			}
			must(shutdown(ctx, u))
		}
	}
	for _, u := range targets {
		must(bootAndAwait(ctx, u))
		must(slimOn(ctx, u))
	}
	for i := 0; i < *iters; i++ {
		for _, u := range targets {
			if err := openURL(ctx, u, deepLink); err == nil {
				res.Seam.SlimOpenURL++
			}
			if _, err := status(ctx, u); err == nil {
				res.Seam.SlimOK++
			}
		}
	}
	for _, u := range targets {
		must(slimOff(ctx, u))
		must(shutdown(ctx, u))
	}
	fmt.Printf("seam: stock %d/%d open_url ok, slim %d/%d\n",
		res.Seam.StockOpenURL, res.Seam.Iterations, res.Seam.SlimOpenURL, res.Seam.Iterations)

	// --- 5. MCP server vs raw CLI: same workflow, two transports. Even when
	// the agent and the simulator share one machine, the question is what a
	// device operation costs through the MCP tool surface (typed contract,
	// single round-trip incl. readiness) vs agent-shelled simctl invocations.
	binary, err := os.Executable()
	if err == nil {
		// The bench binary is not mcp-sim; locate the repo binary instead.
		if _, statErr := os.Stat("bin/mcp-sim"); statErr == nil {
			binary = "bin/mcp-sim"
		}
	}
	if _, err := os.Stat(binary); err == nil {
		mcpSteps, mcpErr := runWorkflowMCP(ctx, binary, targets[0], *iters)
		cliSteps := runWorkflowCLI(ctx, targets[0], *iters)
		res.MCPvsCLI = &workflowResult{MCP: mcpSteps, CLI: cliSteps}
		if mcpErr != nil {
			fmt.Printf("mcp-vs-cli: mcp path error: %v\n", mcpErr)
		} else {
			fmt.Println("mcp :", summarizeSteps(mcpSteps))
		}
		fmt.Println("cli :", summarizeSteps(cliSteps))
		if shape, err := contractShapeCheck(ctx, binary, targets[0]); err == nil {
			res.MCPShape = shape
			fmt.Println("shape:", shape)
		}
		must(shutdown(ctx, targets[0]))
	} else {
		fmt.Printf("mcp-vs-cli skipped: %s not found (run task build)\n", binary)
	}

	must(os.MkdirAll(*outDir, 0o750)) // #nosec G301
	stamp := time.Now().Format("2006-01-02")
	j, _ := json.MarshalIndent(res, "", "  ")
	jsonPath := filepath.Join(*outDir, stamp+".json")
	must(os.WriteFile(jsonPath, j, 0o600))                                                // #nosec G306
	must(os.WriteFile(filepath.Join(*outDir, stamp+".md"), []byte(markdown(res)), 0o600)) // #nosec G306
	fmt.Println("wrote", jsonPath)
}

func collectSpecs(ctx context.Context) machineSpecs {
	var s machineSpecs
	s.Model, _ = sysctl("hw.model")
	s.Chip, _ = sysctl("machdep.cpu.brand_string")
	mem, _ := sysctl("hw.memsize")
	_, _ = fmt.Sscanf(mem, "%d", &s.MemoryGB) //nolint:errcheck
	s.MemoryGB /= 1024 * 1024 * 1024
	s.MacOS, _ = sysctl("kern.osproductversion")
	if out, err := exec.CommandContext(ctx, "simslim", "version").Output(); err == nil {
		s.Simslim = strings.TrimSpace(string(out))
	}
	if out, err := exec.CommandContext(ctx, "xcrun", "simctl", "runtime", "list").Output(); err == nil {
		for _, line := range strings.Split(string(out), "\n") {
			if strings.Contains(line, "iOS") {
				s.Runtime = strings.TrimSpace(line)
				break
			}
		}
	}
	return s
}

func sysctl(name string) (string, error) {
	out, err := exec.Command("sysctl", "-n", name).Output() //nolint:noctx // trivial local probe
	return strings.TrimSpace(string(out)), err
}

func firstIPhones(n int) ([]string, error) {
	ctx := context.Background()
	out, err := exec.CommandContext(ctx, "xcrun", "simctl", "list", "devices", "available").Output()
	if err != nil {
		return nil, err
	}
	var udids []string
	for _, line := range strings.Split(string(out), "\n") {
		if !strings.Contains(line, "iPhone") {
			continue
		}
		if i := strings.Index(line, "("); i > 0 {
			if j := strings.Index(line[i:], ")"); j > 0 {
				udids = append(udids, line[i+1:i+j])
			}
		}
		if len(udids) == n {
			break
		}
	}
	if len(udids) == 0 {
		return nil, fmt.Errorf("no iPhone simulators found")
	}
	return udids, nil
}

func shutdownAll(udids []string) {
	ctx := context.Background()
	for _, u := range udids {
		_ = exec.CommandContext(ctx, "xcrun", "simctl", "shutdown", u).Run()
	}
	time.Sleep(2 * time.Second)
}

func bootAndAwait(ctx context.Context, udid string) error {
	if err := exec.CommandContext(ctx, "xcrun", "simctl", "boot", udid).Run(); err != nil {
		msg := err.Error()
		// "Booted"/exit 149 = device already booted; bootstatus below confirms.
		if !strings.Contains(msg, "already booted") && !strings.Contains(msg, "149") && !strings.Contains(msg, "current state") {
			return fmt.Errorf("boot: %w", err)
		}
	}
	// Wait for boot completion.
	for i := 0; i < 180; i++ {
		out, err := exec.CommandContext(ctx, "xcrun", "simctl", "bootstatus", udid, "-b").CombinedOutput()
		if err == nil {
			_ = out
			return nil
		}
		time.Sleep(time.Second)
	}
	return fmt.Errorf("bootstatus timeout for %s", udid)
}

func shutdown(ctx context.Context, udid string) error {
	return exec.CommandContext(ctx, "xcrun", "simctl", "shutdown", udid).Run()
}

func erase(ctx context.Context, udid string) error {
	_ = shutdown(ctx, udid)
	return exec.CommandContext(ctx, "xcrun", "simctl", "erase", udid).Run()
}

func openURL(ctx context.Context, udid, url string) error {
	return exec.CommandContext(ctx, "xcrun", "simctl", "openurl", udid, url).Run()
}

// slimStatus mirrors simslim status --json.
type slimStatus struct {
	ManagedDisabled int  `json:"managedDisabled"`
	ManagedTotal    int  `json:"managedTotal"`
	Booted          bool `json:"booted"`
	Persistent      bool `json:"persistent"`
}

func (s slimStatus) Slimmed() bool { return s.ManagedDisabled > 0 }

func status(ctx context.Context, udid string) (slimStatus, error) {
	var s slimStatus
	out, err := exec.CommandContext(ctx, "simslim", "status", udid, "--json").Output()
	if err != nil {
		return s, err
	}
	return s, json.Unmarshal(out, &s)
}

func measure(ctx context.Context, udid string) (memSample, error) {
	var m struct {
		Processes int   `json:"processes"`
		Bytes     int64 `json:"bytes"`
	}
	out, err := exec.CommandContext(ctx, "simslim", "measure", udid, "--json").Output()
	if err != nil {
		return memSample{}, err
	}
	err = json.Unmarshal(out, &m)
	return memSample{Bytes: m.Bytes, Processes: m.Processes}, err
}

func swapUsed() int64 {
	out, err := exec.Command("sysctl", "-n", "vm.swapusage").Output() //nolint:noctx // trivial local probe
	if err != nil {
		return 0
	}
	// "total = 1024.00M  used = 312.25M  free = 711.75M (encrypted)"
	parts := strings.Fields(string(out))
	for i, p := range parts {
		if p == "used" && i+2 < len(parts) && parts[i+1] == "=" {
			var v float64
			if _, err := fmt.Sscanf(parts[i+2], "%f", &v); err == nil {
				return int64(v * 1024 * 1024)
			}
		}
	}
	return 0
}

func slimOn(ctx context.Context, udid string) error {
	return exec.CommandContext(ctx, "simslim", "on", udid).Run()
}

func slimOff(ctx context.Context, udid string) error {
	return exec.CommandContext(ctx, "simslim", "off", udid).Run()
}

func short(udid string) string {
	if len(udid) > 8 {
		return udid[:8]
	}
	return udid
}

func must(err error) {
	if err != nil {
		fatal(err)
	}
}

func fatal(err error) {
	fmt.Fprintf(os.Stderr, "bench-slim: %v\n", err)
	os.Exit(1)
}

func markdown(r result) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# bench-slim results %s\n\n", r.Timestamp)
	fmt.Fprintf(&b, "Machine: %s / %s / %d GB RAM / macOS %s / %s / %s\n\n",
		r.Machine.Model, r.Machine.Chip, r.Machine.MemoryGB, r.Machine.MacOS, r.Machine.Runtime, r.Machine.Simslim)

	b.WriteString("## Memory (phys_footprint per simulator)\n\n| State | Sim | Bytes | Processes |\n|---|---|---|---|\n")
	var stock, slim int64
	var stockN, slimN int
	for _, m := range r.Memory {
		fmt.Fprintf(&b, "| %s | %s | %d | %d |\n", m.Label, short(m.UDID), m.Bytes, m.Processes)
		if m.Label == "stock" {
			stock += m.Bytes
			stockN++
		} else {
			slim += m.Bytes
			slimN++
		}
	}
	if stockN > 0 && slimN > 0 {
		fmt.Fprintf(&b, "\nAverage reduction: %.2fx (stock %.2f GB -> slim %.2f GB)\n\n",
			float64(stock)/float64(stockN)/(float64(slim)/float64(slimN)),
			float64(stock)/float64(stockN)/1e9, float64(slim)/float64(slimN)/1e9)
	}

	avg := func(xs []int64) int64 {
		if len(xs) == 0 {
			return 0
		}
		var s int64
		for _, x := range xs {
			s += x
		}
		return s / int64(len(xs))
	}
	b.WriteString("## Boot/ready latency\n\n| Mode | Avg ms | Samples |\n|---|---|---|\n")
	fmt.Fprintf(&b, "| stock | %d | %v |\n", avg(r.Latency.StockBootMS), r.Latency.StockBootMS)
	fmt.Fprintf(&b, "| slim | %d | %v |\n\n", avg(r.Latency.SlimBootMS), r.Latency.SlimBootMS)

	fmt.Fprintf(&b, "## Wipe/re-slim\n\n%d runs, all re-slimmed: %v\n\n", r.WipeRes.Runs, r.WipeRes.AllReslimmed)
	fmt.Fprintf(&b, "## Seam (open_url + get_state)\n\n| Mode | open_url ok | get_state ok |\n|---|---|---|\n")
	fmt.Fprintf(&b, "| stock | %d | %d |\n| slim | %d | %d |\n", r.Seam.StockOpenURL, r.Seam.StockOK, r.Seam.SlimOpenURL, r.Seam.SlimOK)
	b.WriteString("\nRuntime: " + runtime.Version() + "\n")
	return b.String()
}
