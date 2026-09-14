package bootstrap_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
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
// hermetic (no adb/xcrun binaries). Implements AppInstaller/AppLauncher so
// the install_app/launch_app tools are exercised end to end.
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

func (fakePlatform) InstallApp(_ context.Context, target, artifactPath string) error {
	if artifactPath != installedArtifact {
		return errFakeInstall{artifactPath}
	}
	return nil
}

func (fakePlatform) LaunchApp(_ context.Context, target, bundleID string) (int, error) {
	return 4242, nil
}

func (fakePlatform) Capabilities() contract.CapabilitySet {
	return contract.CapabilitiesFor(fakePlatform{})
}

// installedArtifact is the only path the fake installer accepts; tests
// point MCPSIM_ARTIFACT_ROOTS at a temp dir holding it.
var installedArtifact string

type errFakeInstall struct{ got string }

func (e errFakeInstall) Error() string { return "fake install got unexpected artifact " + e.got }

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

	// tools/list returns the full 13-tool surface; all tools register
	// unconditionally regardless of which platforms were detected.
	_, listResult := jsonRPCCall(t, client, jsonRPCRequest(t, mcpURL, sessionID, "tools/list", map[string]any{}))
	tools := toolsByName(t, listResult)
	wantTools := []string{
		"await_ready", "boot_device", "controller_status", "get_state",
		"install_app", "launch_app", "list_devices", "open_url", "start_controller", "stop_controller",
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

// TestInstallLaunchTools exercises the install_app and launch_app dispatch:
// artifact_ref resolution through MCPSIM_ARTIFACT_ROOTS (relative, named,
// and not-found forms) plus the pid round trip, all against the fake
// platform.
func TestInstallLaunchTools(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "app.apk"), []byte("apk"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("MCPSIM_ARTIFACT_ROOTS", dir+":named="+dir)
	installedArtifact = filepath.Join(dir, "app.apk")
	t.Cleanup(func() { installedArtifact = "" })

	ts := startServer(t)
	client := ts.Client()

	// initialize handshake to obtain a session id (jsonRPCCall fatals on
	// non-200, and notifications answer 202, so the handshake uses raw Do).
	initResp, _ := jsonRPCCall(t, client, jsonRPCRequest(t, ts.URL+"/mcp", "", "initialize", map[string]any{
		"protocolVersion": "2024-11-05",
		"capabilities":    map[string]any{},
		"clientInfo":      map[string]any{"name": "test", "version": "0"},
	}))
	defer initResp.Body.Close()
	sessionID := initResp.Header.Get("Mcp-Session-Id")
	notif := jsonRPCRequest(t, ts.URL+"/mcp", sessionID, "notifications/initialized", nil)
	if resp, err := client.Do(notif); err != nil {
		t.Fatalf("notifications/initialized: %v", err)
	} else {
		_ = resp.Body.Close()
	}

	call := func(t *testing.T, name string, args map[string]any) map[string]any {
		t.Helper()
		_, result := jsonRPCCall(t, client, jsonRPCRequest(t, ts.URL+"/mcp", sessionID, "tools/call", map[string]any{
			"name": name, "arguments": args,
		}))
		return result
	}

	// install_app: relative ref resolves under the first root.
	result := call(t, "install_app", map[string]any{
		"platform": "testplat", "target": "dev-1", "artifact_ref": "app.apk",
	})
	if result["structuredContent"] == nil {
		t.Errorf("install_app: missing structuredContent in %v", result)
	}

	// install_app: named artifact:// form.
	result = call(t, "install_app", map[string]any{
		"platform": "testplat", "target": "dev-1", "artifact_ref": "artifact://named/app.apk",
	})
	if result["structuredContent"] == nil {
		t.Errorf("install_app artifact://: missing structuredContent in %v", result)
	}

	// install_app: unresolvable ref surfaces the resolver error.
	result = call(t, "install_app", map[string]any{
		"platform": "testplat", "target": "dev-1", "artifact_ref": "missing/thing.apk",
	})
	if isError, _ := result["isError"].(bool); !isError {
		t.Errorf("install_app missing ref: want isError in result %v", result)
	} else if content := fmt.Sprint(result["content"]); !strings.Contains(content, "did not resolve") {
		t.Errorf("install_app missing ref: unexpected content %s", content)
	}

	// launch_app returns the fake platform's pid.
	result = call(t, "launch_app", map[string]any{
		"platform": "testplat", "target": "dev-1", "bundle_id": "com.example.hello",
	})
	structured, ok := result["structuredContent"].(map[string]any)
	if !ok {
		t.Fatalf("launch_app: missing structuredContent in %v", result)
	}
	if pid, _ := structured["pid"].(float64); int(pid) != 4242 {
		t.Errorf("launch_app pid = %v, want 4242", structured["pid"])
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