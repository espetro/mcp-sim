package agentdevice

import (
	"context"
	"encoding/base64"
	"testing"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

// fakeServer is an in-process MCP server that stands in for the real
// `agent-device mcp` child process. It records the tool calls the adapter
// makes and replays canned responses. Tests connect to it via
// newFakeVerifier, which bypasses exec.Command with an in-memory transport.
type fakeServer struct {
	lastTool string
	lastArgs map[string]any
	snapshot map[string]any
}

// newFakeVerifier wires the adapter's dispatch layer to a fakeServer using
// the SDK's in-memory transports (no child process).
func newFakeVerifier(t *testing.T) (*Verifier, *fakeServer) {
	t.Helper()
	fs := &fakeServer{
		snapshot: map[string]any{
			"nodes": []any{
				map[string]any{"ref": "n1", "role": "Button", "label": "Share", "x": 10.0, "y": 20.0, "width": 100.0, "height": 40.0, "enabled": true},
			},
		},
	}
	server := sdkmcp.NewServer(&sdkmcp.Implementation{Name: "agent-device-fake"}, nil)
	for _, tool := range []string{"tap", "double_tap", "long_press", "swipe", "type", "press_button", "open_url"} {
		name := tool
		sdkmcp.AddTool(server, &sdkmcp.Tool{Name: name}, func(ctx context.Context, _ *sdkmcp.CallToolRequest, in map[string]any) (*sdkmcp.CallToolResult, map[string]any, error) {
			fs.lastTool = name
			fs.lastArgs = in
			return nil, map[string]any{"ok": true}, nil
		})
	}
	sdkmcp.AddTool(server, &sdkmcp.Tool{Name: "snapshot"}, func(ctx context.Context, _ *sdkmcp.CallToolRequest, in map[string]any) (*sdkmcp.CallToolResult, map[string]any, error) {
		fs.lastTool = "snapshot"
		fs.lastArgs = in
		return nil, fs.snapshot, nil
	})
	sdkmcp.AddTool(server, &sdkmcp.Tool{Name: "screenshot"}, func(ctx context.Context, _ *sdkmcp.CallToolRequest, in map[string]any) (*sdkmcp.CallToolResult, map[string]any, error) {
		fs.lastTool = "screenshot"
		fs.lastArgs = in
		return &sdkmcp.CallToolResult{
			Content: []sdkmcp.Content{&sdkmcp.ImageContent{Data: []byte("fakepng"), MIMEType: "image/png"}},
		}, nil, nil
	})

	sTransport, cTransport := sdkmcp.NewInMemoryTransports()
	if _, err := server.Connect(context.Background(), sTransport, nil); err != nil {
		t.Fatalf("server connect: %v", err)
	}
	client := sdkmcp.NewClient(&sdkmcp.Implementation{Name: "mcp-sim-test"}, nil)
	session, err := client.Connect(context.Background(), cTransport, nil)
	if err != nil {
		t.Fatalf("client connect: %v", err)
	}
	t.Cleanup(func() { _ = session.Close() })
	return &Verifier{binPath: "fake", session: session}, fs
}

func TestVerifierTapDispatchesTool(t *testing.T) {
	v, fs := newFakeVerifier(t)

	if err := v.Tap(context.Background(), "ios-dev", "n1"); err != nil {
		t.Fatalf("Tap: %v", err)
	}
	if fs.lastTool != "tap" {
		t.Errorf("tool = %q, want tap", fs.lastTool)
	}
	if fs.lastArgs["device"] != "ios-dev" || fs.lastArgs["ref"] != "n1" {
		t.Errorf("args = %v, want device=ios-dev ref=n1", fs.lastArgs)
	}
}

func TestVerifierTypeDispatchesTool(t *testing.T) {
	v, fs := newFakeVerifier(t)

	if err := v.Type(context.Background(), "emulator-5554", "n2", "hello"); err != nil {
		t.Fatalf("Type: %v", err)
	}
	if fs.lastTool != "type" {
		t.Errorf("tool = %q, want type", fs.lastTool)
	}
	if fs.lastArgs["text"] != "hello" || fs.lastArgs["ref"] != "n2" {
		t.Errorf("args = %v, want text=hello ref=n2", fs.lastArgs)
	}
}

func TestVerifierLongPressDefaultsDuration(t *testing.T) {
	v, fs := newFakeVerifier(t)

	if err := v.LongPress(context.Background(), "d1", "n1", 750); err != nil {
		t.Fatalf("LongPress: %v", err)
	}
	if fs.lastTool != "long_press" || fs.lastArgs["duration_ms"] != float64(750) {
		t.Errorf("tool=%q args=%v", fs.lastTool, fs.lastArgs)
	}
}

func TestVerifierSnapshotParsesNodes(t *testing.T) {
	v, _ := newFakeVerifier(t)

	snap, err := v.Snapshot(context.Background(), "ios-dev")
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}
	if snap.Target != "ios-dev" {
		t.Errorf("target = %q, want ios-dev", snap.Target)
	}
	if len(snap.Nodes) != 1 {
		t.Fatalf("nodes = %d, want 1", len(snap.Nodes))
	}
	n := snap.Nodes[0]
	if n.Ref != "n1" || n.Role != "Button" || n.Label != "Share" || !n.Enabled {
		t.Errorf("unexpected node: %+v", n)
	}
	if n.Width != 100 || n.Height != 40 {
		t.Errorf("frame = %dx%d, want 100x40", n.Width, n.Height)
	}
}

func TestVerifierScreenshotFromImageContent(t *testing.T) {
	v, _ := newFakeVerifier(t)

	shot, err := v.Screenshot(context.Background(), "ios-dev")
	if err != nil {
		t.Fatalf("Screenshot: %v", err)
	}
	if shot.Format != "png" {
		t.Errorf("format = %q, want png", shot.Format)
	}
	got, err := base64.StdEncoding.DecodeString(shot.Base64)
	if err != nil {
		t.Fatalf("base64 decode: %v", err)
	}
	if string(got) != "fakepng" {
		t.Errorf("decoded = %q, want fakepng", got)
	}
}

func TestVerifierPressButtonAndSwipe(t *testing.T) {
	v, fs := newFakeVerifier(t)
	ctx := context.Background()

	if err := v.PressButton(ctx, "d1", "home"); err != nil {
		t.Fatalf("PressButton: %v", err)
	}
	if fs.lastTool != "press_button" || fs.lastArgs["button"] != "home" {
		t.Errorf("tool=%q args=%v", fs.lastTool, fs.lastArgs)
	}
	if err := v.Swipe(ctx, "d1", "up"); err != nil {
		t.Fatalf("Swipe: %v", err)
	}
	if fs.lastTool != "swipe" || fs.lastArgs["direction"] != "up" {
		t.Errorf("tool=%q args=%v", fs.lastTool, fs.lastArgs)
	}
}

func TestVerifierPing(t *testing.T) {
	v, _ := newFakeVerifier(t)
	if err := v.Ping(context.Background()); err != nil {
		t.Errorf("Ping: %v", err)
	}
}

func TestDetectBinaryExplicitPath(t *testing.T) {
	if got := DetectBinary(""); got == "" {
		t.Skip("agent-device not on PATH in test environment")
	}
}

func TestEnsureInstalledMissingBinary(t *testing.T) {
	if _, err := EnsureInstalled("/nonexistent/agent-device-binary-xyz"); err == nil {
		t.Error("want error for missing binary, got nil")
	}
}
