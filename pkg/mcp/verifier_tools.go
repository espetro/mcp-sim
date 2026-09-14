package mcp

import (
	"context"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/espetro/mcp-sim/pkg/contract"
	"github.com/espetro/mcp-sim/pkg/orchestrator"
)

// verifierTargetOnly is the shared input shape of tools that need only a
// device target.
type verifierTargetOnly struct {
	Target string `json:"target"`
}

// registerVerifierTools wires the verifier.* tool family. The tools are
// always advertised; when no verifier backend is configured, calls return a
// structured verifier_unavailable error so agents can fall back or surface a
// setup hint. This matches the graceful-degrade behavior of platforms.
func registerVerifierTools(s *sdkmcp.Server, orch *orchestrator.Orchestrator) {
	sdkmcp.AddTool(s, &sdkmcp.Tool{Name: "verifier_tap", Description: "Tap an element identified by a snapshot ref (from verifier_snapshot)."}, func(ctx context.Context, _ *sdkmcp.CallToolRequest, in struct {
		Target string `json:"target"`
		Ref    string `json:"ref"`
	}) (*sdkmcp.CallToolResult, struct{ Success bool }, error) {
		err := orch.VerifyTap(ctx, in.Target, in.Ref)
		return nil, struct{ Success bool }{Success: err == nil}, err
	})

	sdkmcp.AddTool(s, &sdkmcp.Tool{Name: "verifier_double_tap", Description: "Double-tap an element identified by a snapshot ref."}, func(ctx context.Context, _ *sdkmcp.CallToolRequest, in struct {
		Target string `json:"target"`
		Ref    string `json:"ref"`
	}) (*sdkmcp.CallToolResult, struct{ Success bool }, error) {
		err := orch.VerifyDoubleTap(ctx, in.Target, in.Ref)
		return nil, struct{ Success bool }{Success: err == nil}, err
	})

	sdkmcp.AddTool(s, &sdkmcp.Tool{Name: "verifier_long_press", Description: "Long-press an element identified by a snapshot ref. duration_ms defaults to 500."}, func(ctx context.Context, _ *sdkmcp.CallToolRequest, in struct {
		Target     string `json:"target"`
		Ref        string `json:"ref"`
		DurationMS int    `json:"duration_ms,omitempty"`
	}) (*sdkmcp.CallToolResult, struct{ Success bool }, error) {
		ms := in.DurationMS
		if ms == 0 {
			ms = 500
		}
		err := orch.VerifyLongPress(ctx, in.Target, in.Ref, ms)
		return nil, struct{ Success bool }{Success: err == nil}, err
	})

	sdkmcp.AddTool(s, &sdkmcp.Tool{Name: "verifier_swipe", Description: "Swipe the screen in a direction: up, down, left or right."}, func(ctx context.Context, _ *sdkmcp.CallToolRequest, in struct {
		Target    string `json:"target"`
		Direction string `json:"direction"`
	}) (*sdkmcp.CallToolResult, struct{ Success bool }, error) {
		err := orch.VerifySwipe(ctx, in.Target, in.Direction)
		return nil, struct{ Success bool }{Success: err == nil}, err
	})

	sdkmcp.AddTool(s, &sdkmcp.Tool{Name: "verifier_type", Description: "Type text into an element identified by a snapshot ref."}, func(ctx context.Context, _ *sdkmcp.CallToolRequest, in struct {
		Target string `json:"target"`
		Ref    string `json:"ref"`
		Text   string `json:"text"`
	}) (*sdkmcp.CallToolResult, struct{ Success bool }, error) {
		err := orch.VerifyType(ctx, in.Target, in.Ref, in.Text)
		return nil, struct{ Success bool }{Success: err == nil}, err
	})

	sdkmcp.AddTool(s, &sdkmcp.Tool{Name: "verifier_press_button", Description: "Press a hardware/system button (e.g. home, back, enter)."}, func(ctx context.Context, _ *sdkmcp.CallToolRequest, in struct {
		Target string `json:"target"`
		Button string `json:"button"`
	}) (*sdkmcp.CallToolResult, struct{ Success bool }, error) {
		err := orch.VerifyPressButton(ctx, in.Target, in.Button)
		return nil, struct{ Success bool }{Success: err == nil}, err
	})

	sdkmcp.AddTool(s, &sdkmcp.Tool{Name: "verifier_open_url", Description: "Open a URL or deep link on the device via the verifier backend."}, func(ctx context.Context, _ *sdkmcp.CallToolRequest, in struct {
		Target string `json:"target"`
		URL    string `json:"url"`
	}) (*sdkmcp.CallToolResult, struct{ Success bool }, error) {
		err := orch.VerifyOpenURL(ctx, in.Target, in.URL)
		return nil, struct{ Success bool }{Success: err == nil}, err
	})

	sdkmcp.AddTool(s, &sdkmcp.Tool{Name: "verifier_snapshot", Description: "Capture an accessibility snapshot of the current screen: a list of elements with stable refs for verifier_tap/verifier_type."}, func(ctx context.Context, _ *sdkmcp.CallToolRequest, in verifierTargetOnly) (*sdkmcp.CallToolResult, snapshotResult, error) {
		snap, err := orch.VerifySnapshot(ctx, in.Target)
		if err != nil {
			return nil, snapshotResult{}, err
		}
		return nil, snapshotResult{Snapshot: snap}, nil
	})

	sdkmcp.AddTool(s, &sdkmcp.Tool{Name: "verifier_screenshot", Description: "Capture a screenshot of the current screen as a base64 image."}, func(ctx context.Context, _ *sdkmcp.CallToolRequest, in verifierTargetOnly) (*sdkmcp.CallToolResult, screenshotResult, error) {
		shot, err := orch.VerifyScreenshot(ctx, in.Target)
		if err != nil {
			return nil, screenshotResult{}, err
		}
		return nil, screenshotResult{Screenshot: shot}, nil
	})
}

// snapshotResult is the output shape of verifier_snapshot.
type snapshotResult struct {
	Snapshot contract.Snapshot `json:"snapshot"`
}

// screenshotResult is the output shape of verifier_screenshot.
type screenshotResult struct {
	Screenshot contract.Screenshot `json:"screenshot"`
}
