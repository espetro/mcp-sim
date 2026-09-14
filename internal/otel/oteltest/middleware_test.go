package oteltest

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/espetro/mcp-sim/internal/otel"
	"github.com/espetro/mcp-sim/pkg/mcp"
	"github.com/espetro/mcp-sim/pkg/orchestrator"

	otelapi "go.opentelemetry.io/otel"
	otelsdk "go.opentelemetry.io/otel/sdk/trace"
)

func discardLogger() *slog.Logger { return slog.New(slog.DiscardHandler) }

// startAuditedServer boots the real MCP server over an httptest handler with
// the JSONL exporter installed as a syncer, so a tools/call lands in the
// audit file before the request returns.
func startAuditedServer(t *testing.T) (mcpURL string, auditPath string) {
	t.Helper()
	auditPath = filepath.Join(t.TempDir(), "audit.jsonl")
	exp, err := otel.NewJSONLExporter(auditPath)
	if err != nil {
		t.Fatalf("otel.NewJSONLExporter: %v", err)
	}
	t.Cleanup(func() { _ = exp.Shutdown(context.Background()) })

	tp := otelsdk.NewTracerProvider(otelsdk.WithSyncer(exp))
	prev := otelapi.GetTracerProvider()
	otelapi.SetTracerProvider(tp)
	t.Cleanup(func() {
		otelapi.SetTracerProvider(prev)
		_ = tp.Shutdown(context.Background())
	})

	orch, err := orchestrator.New()
	if err != nil {
		t.Fatalf("orchestrator.New: %v", err)
	}
	srv := mcp.NewServer(orch, discardLogger())
	ts := httptest.NewServer(srv.StreamableHTTPHandlerStateless())
	t.Cleanup(ts.Close)
	return ts.URL + "/mcp", auditPath
}

// rpc performs one JSON-RPC call and decodes the result (SSE unwrapped).
func rpc(t *testing.T, mcpURL, sessionID, method string, params any) (string, map[string]any) {
	t.Helper()
	msg := map[string]any{"jsonrpc": "2.0", "method": method}
	if method != "notifications/initialized" {
		msg["id"] = 1
	}
	if params != nil {
		msg["params"] = params
	}
	body, _ := json.Marshal(msg)
	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, mcpURL, bytes.NewReader(body))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	if sessionID != "" {
		req.Header.Set("Mcp-Session-Id", sessionID)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do %s: %v", method, err)
	}
	defer func() { _ = resp.Body.Close() }()
	buf := new(bytes.Buffer)
	if _, err := buf.ReadFrom(resp.Body); err != nil {
		t.Fatalf("read body: %v", err)
	}
	raw := strings.TrimSpace(buf.String())
	if data, ok := strings.CutPrefix(raw, "data:"); ok {
		raw = strings.TrimSpace(data)
	} else if i := strings.Index(raw, "\ndata:"); i >= 0 {
		raw = strings.TrimSpace(strings.SplitN(raw[i+1:], "data:", 2)[1])
	}
	var out map[string]any
	if raw != "" && raw != "accepted" {
		if err := json.Unmarshal([]byte(raw), &out); err != nil {
			t.Fatalf("decode %s response %q: %v", method, raw, err)
		}
	}
	return resp.Header.Get("Mcp-Session-Id"), out
}

func auditLines(t *testing.T, path string) []map[string]any {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading audit file: %v", err)
	}
	var lines []map[string]any
	for _, l := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		if l == "" {
			continue
		}
		var m map[string]any
		if err := json.Unmarshal([]byte(l), &m); err != nil {
			t.Fatalf("invalid JSONL line: %v", err)
		}
		lines = append(lines, m)
	}
	return lines
}

func findSpan(lines []map[string]any, name string) map[string]any {
	for _, l := range lines {
		if l["name"] == name {
			return l
		}
	}
	return nil
}

func TestDebugExtraLive(t *testing.T) {
	mcpURL, _ := startAuditedServer(t)
	sessionID, _ := rpc(t, mcpURL, "", "initialize", map[string]any{
		"protocolVersion": "2025-03-26",
		"capabilities":    map[string]any{},
		"clientInfo":      map[string]any{"name": "otel-test", "version": "0"},
	})
	rpc(t, mcpURL, sessionID, "notifications/initialized", nil)
	_, _ = rpc(t, mcpURL, sessionID, "tools/call", map[string]any{
		"name":      "list_devices",
		"arguments": map[string]any{},
	})
}

func TestToolCallSpanShape(t *testing.T) {
	mcpURL, auditPath := startAuditedServer(t)

	sessionID, _ := rpc(t, mcpURL, "", "initialize", map[string]any{
		"protocolVersion": "2025-03-26",
		"capabilities":    map[string]any{},
		"clientInfo":      map[string]any{"name": "otel-test", "version": "0"},
	})
	rpc(t, mcpURL, sessionID, "notifications/initialized", nil)
	// list_devices is hermetic: no platform backend needed to succeed or fail.
	_, _ = rpc(t, mcpURL, sessionID, "tools/call", map[string]any{
		"name":      "list_devices",
		"arguments": map[string]any{},
	})

	lines := auditLines(t, auditPath)
	for _, l := range lines {
		if l["schema"] != otel.AuditSchema {
			t.Fatalf("every line must carry the schema header, got %v", l["schema"])
		}
	}
	span := findSpan(lines, "mcp_sim.list_devices")
	if span == nil {
		t.Fatalf("no mcp_sim.list_devices span in %d lines: %v", len(lines), lines)
	}
	attrs := span["attributes"].(map[string]any)
	if attrs["tool.name"] != "list_devices" {
		t.Errorf("tool.name = %v", attrs["tool.name"])
	}
	if attrs["audit.params_sha256"] == "" || len(attrs["audit.params_sha256"].(string)) != 16 {
		t.Errorf("audit.params_sha256 = %v, want 16 hex chars", attrs["audit.params_sha256"])
	}
	if o := attrs["audit.outcome"]; o != "ok" && o != "error" {
		t.Errorf("audit.outcome = %v", o)
	}
	if _, ok := attrs["audit.latency_ms"]; !ok {
		t.Error("audit.latency_ms missing")
	}
	// Raw arguments must never appear in any line.
	if b, _ := json.Marshal(lines); strings.Contains(string(b), "arguments") {
		t.Error("raw arguments leaked into the audit log")
	}
}

func TestTraceparentPropagation(t *testing.T) {
	mcpURL, auditPath := startAuditedServer(t)

	sessionID, _ := rpc(t, mcpURL, "", "initialize", map[string]any{
		"protocolVersion": "2025-03-26",
		"capabilities":    map[string]any{},
		"clientInfo":      map[string]any{"name": "otel-test", "version": "0"},
	})
	rpc(t, mcpURL, sessionID, "notifications/initialized", nil)

	const traceID = "4bf92f3577b34da6a3ce929d0e0e4736"
	const spanID = "00f067aa0ba902b7"
	msg := map[string]any{"jsonrpc": "2.0", "method": "tools/call", "id": 7,
		"params": map[string]any{"name": "list_devices", "arguments": map[string]any{"_marker": "tp"}}}
	body, _ := json.Marshal(msg)
	req, _ := http.NewRequestWithContext(context.Background(), http.MethodPost, mcpURL, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	req.Header.Set("traceparent", "00-"+traceID+"-"+spanID+"-01")
	t.Logf("sending tools/call with tp to %s", mcpURL)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do: %v", err)
	}
	_, _ = io.Copy(io.Discard, resp.Body)
	_ = resp.Body.Close()

	lines := auditLines(t, auditPath)
	span := findSpan(lines, "mcp_sim.list_devices")
	if span == nil {
		t.Fatal("no tool span recorded")
	}
	if span["trace_id"] != traceID {
		t.Errorf("trace_id = %v, want propagated %s", span["trace_id"], traceID)
	}
	if span["parent_span_id"] != spanID {
		t.Errorf("parent_span_id = %v, want %s", span["parent_span_id"], spanID)
	}
}

// TestToolErrorResultOutcome covers tools that fail via an MCP tool error:
// the go-sdk converts a handler error into a CallToolResult with IsError=true
// and nil Go error, so the middleware must still record outcome=error.
func TestToolErrorResultOutcome(t *testing.T) {
	mcpURL, auditPath := startAuditedServer(t)

	sessionID, _ := rpc(t, mcpURL, "", "initialize", map[string]any{
		"protocolVersion": "2025-03-26",
		"capabilities":    map[string]any{},
		"clientInfo":      map[string]any{"name": "otel-test", "version": "0"},
	})
	rpc(t, mcpURL, sessionID, "notifications/initialized", nil)
	// launch_app against a nonexistent platform fails in the orchestrator;
	// the SDK serializes that as isError=true, err=nil.
	_, _ = rpc(t, mcpURL, sessionID, "tools/call", map[string]any{
		"name": "launch_app",
		"arguments": map[string]any{
			"platform":  "nonexistent",
			"target":    "nope",
			"bundle_id": "com.example.app",
		},
	})

	span := findSpan(auditLines(t, auditPath), "mcp_sim.launch_app")
	if span == nil {
		t.Fatal("no mcp_sim.launch_app span in audit log")
	}
	attrs := span["attributes"].(map[string]any)
	if attrs["audit.outcome"] != "error" {
		t.Errorf("audit.outcome = %v, want error", attrs["audit.outcome"])
	}
	code, _ := attrs["audit.error_code"].(string)
	if code == "" {
		t.Error("audit.error_code missing for isError result")
	}
	if status, ok := span["status"].(string); !ok || status != "error: "+code {
		t.Errorf("span status = %v, want error: %s", span["status"], code)
	}
}
