package otel

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	sdkresource "go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

// AuditSchema is the forward-compat header written on every JSONL line.
const AuditSchema = "mcp-sim.audit.v1"

// defaultAuditPath is the default audit log location under the user config
// directory.
func defaultAuditPath() string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return "audit.jsonl"
	}
	return filepath.Join(home, ".config", "mcp-sim", "audit.jsonl")
}

// ResolveAuditPath maps the MCPSIM_AUDIT_LOG value to a concrete sink:
// "stdout", "file" (the default path) or an explicit path.
func ResolveAuditPath(v string) string {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "", "file":
		return defaultAuditPath()
	case "stdout":
		return "stdout"
	default:
		return v
	}
}

// resource returns the OTel resource describing this service.
func resource() *sdkresource.Resource {
	return sdkresource.NewWithAttributes(
		"https://opentelemetry.io/schemas/1.26.0",
		semconvServiceName(),
	)
}

// InitTracer installs the global tracer provider writing spans as JSONL to
// the resolved audit sink. It returns a shutdown closure that flushes
// pending spans and releases the file; call it before process exit.
// When enabled is false it returns a nil shutdown and no-op tracing (the
// default global provider).
func InitTracer(enabled bool, auditPath string) (func(context.Context) error, error) {
	if !enabled {
		return nil, nil
	}
	exp, err := NewJSONLExporter(ResolveAuditPath(auditPath))
	if err != nil {
		return nil, fmt.Errorf("audit exporter: %w", err)
	}
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithResource(resource()),
		sdktrace.WithBatcher(exp,
			sdktrace.WithBatchTimeout(2*time.Second),
			sdktrace.WithMaxExportBatchSize(256),
		),
	)
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagator())
	return tp.Shutdown, nil
}

// syncStdout guards interleaved writes from BatchSpanProcessor goroutines.
type syncStdout struct {
	mu sync.Mutex
}

func (s *syncStdout) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return os.Stdout.Write(p)
}

func (s *syncStdout) Close() error { return nil }

// propagator returns the W3C trace context + baggage propagator used to
// accept traceparent headers from MCP clients.
func propagator() propagation.TextMapPropagator {
	return propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	)
}
