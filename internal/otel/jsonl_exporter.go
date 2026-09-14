package otel

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"gopkg.in/natefinch/lumberjack.v2"
)

// semconvServiceName pins the service.name resource attribute.
func semconvServiceName() attribute.KeyValue {
	return semconv.ServiceName("mcp-sim")
}

// JSONLExporter is an OTel SDK SpanExporter that writes one JSON object per
// line to an append-only file (rotated via lumberjack) or stdout.
type JSONLExporter struct {
	mu sync.Mutex
	w  io.WriteCloser
}

// NewJSONLExporter opens the sink at path ("stdout" for standard output).
// File sinks rotate at 50 MB with 5 generations kept.
func NewJSONLExporter(path string) (*JSONLExporter, error) {
	if path == "stdout" {
		return &JSONLExporter{w: &syncStdout{}}, nil
	}
	if dir := filepath.Dir(path); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return nil, fmt.Errorf("creating audit dir: %w", err)
		}
	}
	// Provoke lumberjack into creating the file eagerly so operators (and
	// tests) can rely on the sink existing after startup.
	if f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600); err == nil {
		_ = f.Close()
	}
	lj := &lumberjack.Logger{
		Filename:   path,
		MaxSize:    50, // MB
		MaxBackups: 5,
		MaxAge:     0,
		Compress:   false,
	}
	return &JSONLExporter{w: lj}, nil
}

// auditLine is the on-disk shape: a schema header plus the span payload
// with audit.* attributes promoted to top-level keys.
type auditLine struct {
	Schema  string            `json:"schema"`
	Name    string            `json:"name"`
	Start   time.Time         `json:"start_time"`
	End     time.Time         `json:"end_time"`
	TraceID string            `json:"trace_id"`
	SpanID  string            `json:"span_id"`
	Parent  string            `json:"parent_span_id,omitempty"`
	Status  string            `json:"status"`
	Attrs   map[string]string `json:"attributes"`
	Audit   map[string]string `json:"audit,omitempty"`
}

func (e *JSONLExporter) ExportSpans(ctx context.Context, spans []sdktrace.ReadOnlySpan) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	enc := json.NewEncoder(e.w)
	for _, s := range spans {
		attrs := flattenAttrs(s.Attributes())
		audit := map[string]string{}
		for k, v := range attrs {
			if strings.HasPrefix(k, "audit.") {
				audit[k] = v
			}
		}
		line := auditLine{
			Schema:  AuditSchema,
			Name:    s.Name(),
			Start:   s.StartTime(),
			End:     s.EndTime(),
			TraceID: s.SpanContext().TraceID().String(),
			SpanID:  s.SpanContext().SpanID().String(),
			Parent:  s.Parent().SpanID().String(),
			Status:  statusString(s.Status()),
			Attrs:   attrs,
			Audit:   audit,
		}
		if len(line.Audit) == 0 {
			line.Audit = nil
		}
		if err := enc.Encode(line); err != nil {
			return fmt.Errorf("encoding audit span: %w", err)
		}
	}
	return nil
}

// Shutdown flushes and closes the underlying sink.
func (e *JSONLExporter) Shutdown(ctx context.Context) error { return e.w.Close() }

var _ sdktrace.SpanExporter = (*JSONLExporter)(nil)

func statusString(s sdktrace.Status) string {
	switch s.Code {
	case codes.Error:
		if s.Description != "" {
			return "error: " + s.Description
		}
		return "error"
	case codes.Ok:
		return "ok"
	default:
		return "unset"
	}
}

func flattenAttrs(kvs []attribute.KeyValue) map[string]string {
	out := make(map[string]string, len(kvs))
	for _, kv := range kvs {
		out[string(kv.Key)] = kv.Value.Emit()
	}
	return out
}
