package otel

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

// readLines parses a JSONL audit file into a slice of maps.
func readLines(t *testing.T, path string) []map[string]any {
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
			t.Fatalf("line is not valid JSON (%q): %v", l, err)
		}
		lines = append(lines, m)
	}
	return lines
}

func TestJSONLExporterWritesValidJSONL(t *testing.T) {
	path := filepath.Join(t.TempDir(), "audit.jsonl")
	exp, err := NewJSONLExporter(path)
	if err != nil {
		t.Fatalf("NewJSONLExporter: %v", err)
	}

	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exp))
	prev := otel.GetTracerProvider()
	otel.SetTracerProvider(tp)
	defer otel.SetTracerProvider(prev)

	tracer := otel.Tracer(TracerName)
	ctx, parent := tracer.Start(context.Background(), "mcp_sim.parent")
	_, child := tracer.Start(ctx, "mcp_sim.install_app")
	child.SetAttributes(strAttrs(
		"tool.name", "install_app",
		"audit.outcome", "ok",
		"audit.params_sha256", "8a3f1c2d9b7e4a01",
	)...)
	child.End()
	parent.End()

	if err := tp.Shutdown(context.Background()); err != nil {
		t.Fatalf("shutdown: %v", err)
	}
	if err := exp.Shutdown(context.Background()); err != nil {
		t.Fatalf("exporter shutdown: %v", err)
	}

	lines := readLines(t, path)
	if len(lines) != 2 {
		t.Fatalf("want 2 lines, got %d", len(lines))
	}
	for _, l := range lines {
		if l["schema"] != AuditSchema {
			t.Errorf("line missing schema header: %v", l["schema"])
		}
	}
	childLine := lines[0]
	if childLine["name"] != "mcp_sim.install_app" {
		t.Errorf("span name = %v", childLine["name"])
	}
	if childLine["parent_span_id"] == "" || childLine["parent_span_id"] == childLine["span_id"] {
		t.Errorf("expected a real parent span id, got %v", childLine["parent_span_id"])
	}
	attrs, _ := childLine["attributes"].(map[string]any)
	if attrs["tool.name"] != "install_app" || attrs["audit.outcome"] != "ok" {
		t.Errorf("attributes missing: %v", attrs)
	}
}

func TestNewJSONLExporterCreatesDir(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "dir", "audit.jsonl")
	exp, err := NewJSONLExporter(path)
	if err != nil {
		t.Fatalf("NewJSONLExporter: %v", err)
	}
	if err := exp.Shutdown(context.Background()); err != nil {
		t.Fatalf("shutdown: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Errorf("expected file created at %s: %v", path, err)
	}
}

func TestResolveAuditPath(t *testing.T) {
	cases := []struct{ in, want string }{
		{"", "config"}, // default: resolved to config dir (ends with audit.jsonl)
		{"file", "config"},
		{"stdout", "stdout"},
		{"/tmp/x/audit.jsonl", "/tmp/x/audit.jsonl"},
	}
	for _, c := range cases {
		got := ResolveAuditPath(c.in)
		if c.want == "config" {
			if !strings.HasSuffix(got, "audit.jsonl") {
				t.Errorf("ResolveAuditPath(%q) = %q, want default audit.jsonl", c.in, got)
			}
			continue
		}
		if got != c.want {
			t.Errorf("ResolveAuditPath(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func strAttrs(kv ...string) []attribute.KeyValue {
	var out []attribute.KeyValue
	for i := 0; i+1 < len(kv); i += 2 {
		out = append(out, attribute.String(kv[i], kv[i+1]))
	}
	return out
}

func TestDisabledInitIsNoop(t *testing.T) {
	shutdown, err := InitTracer(false, "stdout")
	if err != nil {
		t.Fatalf("InitTracer disabled: %v", err)
	}
	if shutdown != nil {
		t.Error("disabled init must return nil shutdown")
	}
	tracer := otel.Tracer(TracerName)
	_, span := tracer.Start(context.Background(), "mcp_sim.nothing")
	if span.SpanContext().TraceID().IsValid() && span.IsRecording() {
		t.Error("disabled mode must not record spans")
	}
	span.End()
}

func TestInitTracerStdoutShutdown(t *testing.T) {
	shutdown, err := InitTracer(true, "stdout")
	if err != nil {
		t.Fatalf("InitTracer: %v", err)
	}
	if shutdown == nil {
		t.Fatal("enabled init must return a shutdown closure")
	}
	if err := shutdown(context.Background()); err != nil {
		t.Fatalf("shutdown: %v", err)
	}
}
