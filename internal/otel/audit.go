// Audit attribute helpers: span naming, the params hash and the standard
// attribute set carried by every tool-call span.
package otel

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"

	"go.opentelemetry.io/otel/attribute"
)

// TracerName is the instrumentation name used for all mcp-sim spans.
const TracerName = "github.com/espetro/mcp-sim"

// SpanName returns the span name for a tool call: "mcp_sim.<tool>".
func SpanName(tool string) string { return "mcp_sim." + tool }

// HashParams returns the first 16 hex chars of the SHA-256 of the params
// JSON (encoding/json sorts map keys, so the hash is stable). Raw params
// are never emitted anywhere.
func HashParams(params any) string {
	b, err := json.Marshal(params)
	if err != nil {
		b = []byte("<unmarshalable>")
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])[:16]
}

// Attrs builds the standard attribute set for a tool-call span.
// extra carries per-call attributes (e.g. tool.platform, tool.target).
func Attrs(tool string, params any, extra ...attribute.KeyValue) []attribute.KeyValue {
	attrs := append([]attribute.KeyValue{
		attribute.String("tool.name", tool),
		attribute.String("audit.params_sha256", HashParams(params)),
	}, extra...)
	return attrs
}

// CallerIPFromContext extracts tool.caller_ip placed on the context by the
// HTTP transport, if any.
func CallerIPFromContext(ctx context.Context) (string, bool) {
	ip, ok := ctx.Value(callerIPKey{}).(string)
	return ip, ok && ip != ""
}

type callerIPKey struct{}

// WithCallerIP stores the caller IP on the context.
func WithCallerIP(ctx context.Context, ip string) context.Context {
	return context.WithValue(ctx, callerIPKey{}, ip)
}
