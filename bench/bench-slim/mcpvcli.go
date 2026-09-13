// MCP-vs-CLI comparison for the bench matrix.
//
// Measures the same device workflow twice:
//
//  1. Via mcp-sim: a real MCP client session (stdio) against `mcp-sim mcp`,
//     using the boot_device / await_ready / get_state / open_url / wipe_device
//     tool calls.
//  2. Via raw CLI: the equivalent `xcrun simctl` commands an agent would
//     shell out to without mcp-sim (boot, bootstatus, list devices for state,
//     openurl, erase), plus `simslim on` for the slim variant.
//
// For each step we record wall-clock ms and the size of the data returned to
// the caller (bytes the agent must ingest). The MCP path also captures how
// many structured fields come back vs raw text.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/espetro/mcp-sim/pkg/contract"
	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

// stepResult is one workflow step for one mode.
type stepResult struct {
	Step      string `json:"step"`        // boot / await / state / open_url / wipe
	Mode      string `json:"mode"`        // mcp / cli
	MillisMS  int64  `json:"ms"`
	BytesBack int    `json:"bytes_back"` // response size the caller ingests
	OK        bool   `json:"ok"`
}

// workflowResult aggregates per-mode step timings.
type workflowResult struct {
	MCP []stepResult `json:"mcp"`
	CLI []stepResult `json:"cli"`
}

// runWorkflowMCP drives the workflow through a real MCP stdio session.
func runWorkflowMCP(ctx context.Context, binary, udid string, iters int) ([]stepResult, error) {
	cmd := exec.CommandContext(ctx, binary, "mcp") //nolint:noctx // session-managed lifetime
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	client := sdk.NewClient(&sdk.Implementation{Name: "bench-slim", Version: "0.0.1"}, nil)
	sess, err := client.Connect(ctx, &sdk.CommandTransport{Command: cmd}, nil)
	if err != nil {
		return nil, fmt.Errorf("mcp connect: %w (stderr: %s)", err, stderr.String())
	}
	defer func() { _ = sess.Close() }()

	call := func(step, tool string, args map[string]any) stepResult {
		start := time.Now()
		raw, _ := json.Marshal(args)
		res, err := sess.CallTool(ctx, &sdk.CallToolParams{
			Name:      tool,
			Arguments: args,
		})
		r := stepResult{Step: step, Mode: "mcp", MillisMS: time.Since(start).Milliseconds(), OK: err == nil}
		if res != nil {
			if b, jerr := json.Marshal(res.Content); jerr == nil {
				r.BytesBack = len(b)
			}
		}
		_ = raw
		return r
	}

	var steps []stepResult
	for i := 0; i < iters; i++ {
		steps = append(steps, call("boot", "boot_device", map[string]any{"platform": "ios", "target": udid}))
		steps = append(steps, call("await", "await_ready", map[string]any{"platform": "ios", "target": udid, "timeout": 180}))
		steps = append(steps, call("state", "get_state", map[string]any{"platform": "ios", "target": udid}))
		steps = append(steps, call("open_url", "open_url", map[string]any{"platform": "ios", "target": udid, "url": "https://www.apple.com"}))
		steps = append(steps, call("wipe", "wipe_device", map[string]any{"platform": "ios", "target": udid}))
	}
	return steps, nil
}

// runWorkflowCLI drives the same workflow with raw simctl invocations.
func runWorkflowCLI(ctx context.Context, udid string, iters int) []stepResult {
	timed := func(step string, fn func() ([]byte, error)) stepResult {
		start := time.Now()
		out, err := fn()
		return stepResult{
			Step:      step,
			Mode:      "cli",
			MillisMS:  time.Since(start).Milliseconds(),
			BytesBack: len(out),
			OK:        err == nil,
		}
	}

	run := func(args ...string) ([]byte, error) {
		c := exec.CommandContext(ctx, args[0], args[1:]...)
		return c.CombinedOutput()
	}

	var steps []stepResult
	for i := 0; i < iters; i++ {
		steps = append(steps, timed("boot", func() ([]byte, error) {
			return run("xcrun", "simctl", "boot", udid)
		}))
		steps = append(steps, timed("await", func() ([]byte, error) {
			return run("xcrun", "simctl", "bootstatus", udid, "-b")
		}))
		steps = append(steps, timed("state", func() ([]byte, error) {
			return run("xcrun", "simctl", "list", "devices", "booted", "--json")
		}))
		steps = append(steps, timed("open_url", func() ([]byte, error) {
			return run("xcrun", "simctl", "openurl", udid, "https://www.apple.com")
		}))
		steps = append(steps, timed("wipe", func() ([]byte, error) {
			_, _ = run("xcrun", "simctl", "shutdown", udid)
			return run("xcrun", "simctl", "erase", udid)
		}))
	}
	return steps
}

// summarizeSteps averages ms per step name per mode.
func summarizeSteps(steps []stepResult) map[string]string {
	sum := map[string]int64{}
	cnt := map[string]int{}
	ok := map[string]int{}
	for _, s := range steps {
		sum[s.Step] += s.MillisMS
		cnt[s.Step]++
		if s.OK {
			ok[s.Step]++
		}
	}
	out := map[string]string{}
	for step, total := range sum {
		out[step] = fmt.Sprintf("%dms avg, %d/%d ok, ~%d B back",
			total/int64(cnt[step]), ok[step], cnt[step], bytesBack(steps, step))
	}
	return out
}

func bytesBack(steps []stepResult, step string) int {
	var total, n int
	for _, s := range steps {
		if s.Step == step {
			total += s.BytesBack
			n++
		}
	}
	if n == 0 {
		return 0
	}
	return total / n
}

// contractShapeCheck verifies the MCP surface returns structured typed data
// (the contract.Device shape) rather than free-form CLI text.
func contractShapeCheck(ctx context.Context, binary, udid string) (string, error) {
	cmd := exec.CommandContext(ctx, binary, "mcp") //nolint:noctx // session-managed lifetime
	client := sdk.NewClient(&sdk.Implementation{Name: "bench-slim", Version: "0.0.1"}, nil)
	sess, err := client.Connect(ctx, &sdk.CommandTransport{Command: cmd}, nil)
	if err != nil {
		return "", err
	}
	defer func() { _ = sess.Close() }()

	res, err := sess.CallTool(ctx, &sdk.CallToolParams{
		Name:      "boot_device",
		Arguments: map[string]any{"platform": "ios", "target": udid},
	})
	if err != nil {
		return "", err
	}
	var dev contract.Device
	if res.StructuredContent != nil {
		b, _ := json.Marshal(res.StructuredContent)
		if jerr := json.Unmarshal(b, &dev); jerr == nil && dev.ID == udid {
			return fmt.Sprintf("typed contract.Device: id=%s state=%s platform=%s", dev.ID, dev.State, dev.Platform), nil
		}
	}
	return "", fmt.Errorf("unstructured response")
}

// unused import guards
var _ = strings.TrimSpace
var _ = contract.Device{}
