package bootstrap_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/espetro/mcp-sim/internal/bootstrap"
	"github.com/espetro/mcp-sim/internal/config"
	"github.com/espetro/mcp-sim/pkg/contract"
)

const testToken = "test-bearer-token-0123456789abcdef"

// fakePlatform is a minimal contract.Platform keeping the integration test
// hermetic (no adb/xcrun binaries).
type fakePlatform struct{}

func (fakePlatform) Name() string { return "testplat" }

func (fakePlatform) List(context.Context) ([]contract.Device, error) {
	return []contract.Device{{Platform: "testplat", ID: "dev-1", State: contract.DeviceStateStopped}}, nil
}

func (fakePlatform) Start(_ context.Context, target string, _ contract.StartOpts) (contract.Device, error) {
	return contract.Device{Platform: "testplat", ID: target, State: contract.DeviceStateRunning}, nil
}

func (fakePlatform) Stop(context.Context, string) error { return nil }

func (fakePlatform) State(context.Context, string) (contract.DeviceState, error) {
	return contract.DeviceStateStopped, nil
}

func (fakePlatform) AwaitReady(context.Context, string, time.Duration) error { return nil }

func (fakePlatform) Wipe(context.Context, string) error { return nil }

func (fakePlatform) OpenURL(context.Context, string, string) error { return nil }

func (fakePlatform) Capabilities() contract.CapabilitySet { return contract.CapAll }

// hermeticConfig returns a Config with every platform and controller
// disabled: BuildOrchestrator probes no binaries. Listen is ":0"; the
// actual address comes from httptest.NewServer.
func hermeticConfig() config.Config {
	cfg := config.Config{Server: config.ServerConfig{
		Listen:    ":0",
		LogLevel:  "info",
		LogFormat: "text",
		Auth:      config.AuthConfig{Enabled: true, Token: testToken},
	}}
	cfg.Platforms.IOS.Enabled = false
	cfg.Platforms.Android.Enabled = false
	cfg.Controllers.AgentDevice.Enabled = false
	return cfg
}

// startServer builds the real HTTP wiring via BuildHTTPServerWithAuth,
// registers a fake platform, and serves it through httptest.
func startServer(t *testing.T) *httptest.Server {
	t.Helper()
	logger := slog.New(slog.DiscardHandler)
	orch, srv, err := bootstrap.BuildHTTPServerWithAuth(context.Background(), hermeticConfig(), logger, testToken)
	if err != nil {
		t.Fatalf("BuildHTTPServerWithAuth: %v", err)
	}
	if err := orch.RegisterPlatform(fakePlatform{}); err != nil {
		t.Fatalf("RegisterPlatform: %v", err)
	}
	ts := httptest.NewServer(srv.Handler)
	t.Cleanup(ts.Close)
	return ts
}

// jsonRPCRequest builds a POST request against the streamable endpoint with
// the headers the transport requires.
func jsonRPCRequest(t *testing.T, mcpURL, sessionID, method string, params any) *http.Request {
	t.Helper()
	msg := map[string]any{"jsonrpc": "2.0", "method": method}
	if params != nil {
		msg["params"] = params
	}
	if method != "notifications/initialized" {
		msg["id"] = 1
	}
	body, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("marshal %s: %v", method, err)
	}
	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, mcpURL, bytes.NewReader(body))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	req.Header.Set("Authorization", "Bearer "+testToken)
	if sessionID != "" {
		req.Header.Set("Mcp-Session-Id", sessionID)
	}
	return req
}

// jsonRPCCall performs a request and returns the response plus the decoded
// JSON-RPC result. SSE bodies ("data: <json>") are unwrapped. The response
// body is drained; the caller may still read headers.
func jsonRPCCall(t *testing.T, client *http.Client, req *http.Request) (*http.Response, map[string]any) {
	t.Helper()
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("do request: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, body = %s", resp.StatusCode, raw)
	}
	trimmed := bytes.TrimSpace(raw)
	if bytes.HasPrefix(trimmed, []byte("event:")) || bytes.HasPrefix(trimmed, []byte("data:")) {
		for _, line := range strings.Split(string(trimmed), "\n") {
			if data, ok := strings.CutPrefix(line, "data:"); ok {
				trimmed = []byte(strings.TrimSpace(data))
				break
			}
		}
	}
	var envelope struct {
		Result map[string]any `json:"result"`
		Error  *struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(trimmed, &envelope); err != nil {
		t.Fatalf("decode response %q: %v", trimmed, err)
	}
	if envelope.Error != nil {
		t.Fatalf("JSON-RPC error %d: %s", envelope.Error.Code, envelope.Error.Message)
	}
	return resp, envelope.Result
}

// TestStreamableHTTPAuthFlow exercises the streamable transport behind the
// real bearer middleware: 401 challenge, initialize with session capture,
// notifications/initialized, tools/list, and tools/call get_state.
func TestStreamableHTTPAuthFlow(t *testing.T) {
	ts := startServer(t)
	client := ts.Client()
	mcpURL := ts.URL + "/mcp"

	// (1) POST without Authorization: 401 with a Bearer challenge.
	noAuth, err := http.NewRequestWithContext(context.Background(), http.MethodPost, mcpURL,
		strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	noAuth.Header.Set("Content-Type", "application/json")
	noAuth.Header.Set("Accept", "application/json, text/event-stream")
	resp, err := client.Do(noAuth)
	if err != nil {
		t.Fatalf("unauthenticated POST: %v", err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("no auth: status = %d, want 401", resp.StatusCode)
	}
	if challenge := resp.Header.Get("WWW-Authenticate"); !strings.Contains(challenge, "Bearer") {
		t.Errorf("WWW-Authenticate = %q, want Bearer challenge", challenge)
	}

	// (2) initialize with a Bearer token, capturing the session header.
	initReq := jsonRPCRequest(t, mcpURL, "", "initialize", map[string]any{
		"protocolVersion": "2024-11-05",
		"capabilities":    map[string]any{},
		"clientInfo":      map[string]any{"name": "bootstrap-test", "version": "0.0.1"},
	})
	initResp, initResult := jsonRPCCall(t, client, initReq)
	sessionID := initResp.Header.Get("Mcp-Session-Id")
	if sessionID == "" {
		t.Fatal("initialize: missing Mcp-Session-Id response header")
	}
	if serverInfo, ok := initResult["serverInfo"].(map[string]any); !ok || serverInfo["name"] != "mcp-sim" {
		t.Errorf("initialize serverInfo = %v, want name mcp-sim", initResult["serverInfo"])
	}

	// notifications/initialized closes the handshake (202 Accepted).
	notifReq := jsonRPCRequest(t, mcpURL, sessionID, "notifications/initialized", nil)
	notifResp, err := client.Do(notifReq)
	if err != nil {
		t.Fatalf("notifications/initialized: %v", err)
	}
	_ = notifResp.Body.Close()
	if notifResp.StatusCode != http.StatusAccepted && notifResp.StatusCode != http.StatusOK {
		t.Errorf("notifications/initialized: status = %d, want 202 or 200", notifResp.StatusCode)
	}

	// tools/list returns the full 11-tool surface; all tools register
	// unconditionally regardless of which platforms were detected.
	_, listResult := jsonRPCCall(t, client, jsonRPCRequest(t, mcpURL, sessionID, "tools/list", map[string]any{}))
	tools := toolsByName(t, listResult)
	wantTools := []string{
		"await_ready", "boot_device", "controller_status", "get_state",
		"list_devices", "open_url", "start_controller", "stop_controller",
		"stop_device", "stream_info", "wipe_device",
	}
	if len(tools) != len(wantTools) {
		t.Errorf("tools/list returned %d tools, want %d", len(tools), len(wantTools))
	}
	for _, name := range wantTools {
		if _, ok := tools[name]; !ok {
			t.Errorf("tools/list missing tool %q", name)
		}
	}

	// tools/call get_state returns the fake platform's state inside the
	// tool's structured content envelope.
	_, callResult := jsonRPCCall(t, client, jsonRPCRequest(t, mcpURL, sessionID, "tools/call", map[string]any{
		"name":      "get_state",
		"arguments": map[string]any{"platform": "testplat", "target": "dev-1"},
	}))
	structured, ok := callResult["structuredContent"].(map[string]any)
	if !ok {
		t.Fatalf("tools/call get_state: missing structuredContent in result %v", callResult)
	}
	if structured["state"] != string(contract.DeviceStateStopped) {
		t.Errorf("get_state state = %v, want %q", structured["state"], contract.DeviceStateStopped)
	}
}

// TestHealthzExempt verifies /healthz answers 200 without any token.
func TestHealthzExempt(t *testing.T) {
	ts := startServer(t)
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, ts.URL+"/healthz", nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	resp, err := ts.Client().Do(req)
	if err != nil {
		t.Fatalf("healthz: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("healthz: status = %d, want 200", resp.StatusCode)
	}
	if strings.TrimSpace(string(body)) != "ok" {
		t.Errorf("healthz body = %q, want ok", body)
	}
}

// TestProtectedResourceMetadata verifies the RFC 9728 document is served
// unauthenticated with the header bearer method and a resource field.
func TestProtectedResourceMetadata(t *testing.T) {
	ts := startServer(t)
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, ts.URL+"/.well-known/oauth-protected-resource", nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	resp, err := ts.Client().Do(req)
	if err != nil {
		t.Fatalf("metadata: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("metadata: status = %d, want 200", resp.StatusCode)
	}
	var doc struct {
		Resource               string   `json:"resource"`
		BearerMethodsSupported []string `json:"bearer_methods_supported"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&doc); err != nil {
		t.Fatalf("metadata: decode: %v", err)
	}
	if len(doc.BearerMethodsSupported) == 0 || !slicesContains(doc.BearerMethodsSupported, "header") {
		t.Errorf("bearer_methods_supported = %v, want to include header", doc.BearerMethodsSupported)
	}
	if !strings.HasSuffix(doc.Resource, "/mcp") {
		t.Errorf("resource = %q, want the /mcp endpoint URL", doc.Resource)
	}
}

// toolsByName flattens a tools/list result into a name -> tool map.
func toolsByName(t *testing.T, result map[string]any) map[string]any {
	t.Helper()
	tools, ok := result["tools"].([]any)
	if !ok {
		t.Fatalf("tools/list result missing tools array: %v", result)
	}
	out := make(map[string]any, len(tools))
	for _, raw := range tools {
		tool, ok := raw.(map[string]any)
		if !ok {
			t.Fatalf("tool entry not an object: %v", raw)
		}
		name, _ := tool["name"].(string)
		out[name] = tool
	}
	return out
}

func slicesContains(list []string, want string) bool {
	for _, v := range list {
		if v == want {
			return true
		}
	}
	return false
}