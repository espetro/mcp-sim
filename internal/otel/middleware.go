package otel

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	oteltrace "go.opentelemetry.io/otel/trace"
)

// Aliases over the SDK middleware types so the signature reads locally.
type (
	mcpRequest          = sdkmcp.Request
	mcpResult           = sdkmcp.Result
	sdkmcpMethodHandler = sdkmcp.MethodHandler
)

// propagationTraceContext returns the W3C TraceContext propagator directly,
// so propagation works even before InitTracer installs the global
// propagator (e.g. in tests).
func propagationTraceContext() propagation.TextMapPropagator {
	return propagation.TraceContext{}
}

// ingressTracer returns the tracer for ingress spans.
func ingressTracer() oteltrace.Tracer { return otel.Tracer(TracerName) }

// WithTracing wraps the /mcp HTTP handler in an outermost ingress span and
// accepts W3C traceparent headers from clients (so a CI runner that starts
// a trace sees mcp-sim's spans as children). Disabled mode stays a no-op:
// the default global provider drops everything.
func WithTracing(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := propagationTraceContext().Extract(r.Context(), propagation.HeaderCarrier(r.Header))
		// Carry the inbound trace context and caller IP into the MCP request
		// handling: the streamable transport passes req.Context() to the
		// session, and the middleware below reads these values.
		ctx = WithCallerIP(ctx, callerIP(r))
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// callerIP best-effort extracts the client IP from the transport.
func callerIP(r *http.Request) string {
	if fwd := r.Header.Get("X-Forwarded-For"); fwd != "" {
		if i := strings.IndexByte(fwd, ','); i >= 0 {
			return strings.TrimSpace(fwd[:i])
		}
		return strings.TrimSpace(fwd)
	}
	if host := r.RemoteAddr; host != "" {
		if i := strings.LastIndexByte(host, ':'); i > 0 {
			return host[:i]
		}
		return host
	}
	return ""
}

// MCPMiddleware is an MCP receiving middleware: every MCP request gets a
// span; tools/call additionally gets the standard audit attribute set
// (tool.name, audit.params_sha256, audit.outcome, audit.latency_ms). This
// is the single funnel for tool spans, so new tools are covered
// automatically.
func MCPMiddleware() func(sdkmcpMethodHandler) sdkmcpMethodHandler {
	return func(next sdkmcpMethodHandler) sdkmcpMethodHandler {
		return func(ctx context.Context, method string, req mcpRequest) (mcpResult, error) {
			// Accept a traceparent supplied on the MCP request itself (the
			// streamable transport copies request headers onto Extra.Header),
			// so clients can parent mcp-sim spans into their own trace. This
			// runs before the span starts, which re-parents it naturally.
			if extra := req.GetExtra(); extra != nil && extra.Header != nil {
				ctx = propagationTraceContext().Extract(ctx, propagation.HeaderCarrier(extra.Header))
			}
			ctx, span := ingressTracer().Start(ctx, "mcp."+method)
			defer span.End()

			if method == "tools/call" {
				name, args := toolCallInfo(req)
				if name != "" {
					span.SetName(SpanName(name))
					span.SetAttributes(Attrs(name, args)...)
					if ip, ok := CallerIPFromContext(ctx); ok {
						span.SetAttributes(attribute.String("tool.caller_ip", ip))
					}
				}
			}

			start := time.Now()
			res, err := next(ctx, method, req)
			span.SetAttributes(attribute.Int64("audit.latency_ms", time.Since(start).Milliseconds()))
			if err != nil {
				span.SetAttributes(
					attribute.String("audit.outcome", "error"),
					attribute.String("audit.error_code", err.Error()),
				)
				span.SetStatus(codes.Error, err.Error())
			} else {
				span.SetAttributes(attribute.String("audit.outcome", "ok"))
			}
			return res, err
		}
	}
}

// toolCallInfo extracts the tool name and raw arguments from a tools/call
// request. Args stay opaque (json.RawMessage) so they are hashed, never
// logged or unmarshaled.
func toolCallInfo(req sdkmcp.Request) (name string, args any) {
	switch p := req.GetParams().(type) {
	case *sdkmcp.CallToolParamsRaw:
		return p.Name, json.RawMessage(p.Arguments)
	case *sdkmcp.CallToolParams:
		return p.Name, p.Arguments
	default:
		return "", nil
	}
}
