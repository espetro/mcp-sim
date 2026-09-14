# OTel observability implementation notes (2026-09-14)

Shipped the OTel spine + JSONL audit exporter in one pass (no phased
slog-first work). Deviations from the original plan sketch, discovered
while wiring:

- **No `internal/orchestrator/dispatch.go` funnel exists.** Tools are
  typed `AddTool` closures in `pkg/mcp/server.go`. Instead of touching
  every closure, the span funnel is ONE receiving middleware
  (`otel.MCPMiddleware`) registered via `s.AddReceivingMiddleware` in
  `NewServer` plus `otel.WithTracing` around the streamable HTTP handler.
  New tools get spans for free.
- **go-sdk v1.6 `ServerOptions` has no middleware field**; middleware is
  added post-construction via `AddReceivingMiddleware`. Request params
  arrive as `*sdkmcp.ServerRequest[*sdkmcp.CallToolParamsRaw]` (a
  concrete generic, not the `Request` interface payload of the older
  sketch) - type-switch accordingly.
- **Stateful streamable transport detaches the HTTP request context**
  after `initialize` (jsonrpc2 runs on the session's long-lived ctx), so
  HTTP-header traceparent does NOT reach tools/call spans on the shared
  session. Fixes: (a) the streamable handler copies request headers onto
  `req.Extra.Header`, so the middleware re-extracts W3C context from
  there per request; (b) `WithTracing` still extracts on the HTTP ctx for
  stateless mode. The MCP go-sdk also runs handlers on jsonrpc2's
  connection goroutine, not the HTTP one.
- **`otel.GetTextMapPropagator()` is empty until someone installs one.**
  Our middleware uses `propagation.TraceContext{}` directly so
  propagation works even in tests / disabled mode.
- **JSONL exporter + `WithSyncer` in tests**: use `sdktrace.WithSyncer`
  (not the batcher) in unit tests so spans hit the file before
  assertions; production uses `WithBatcher` (2s timeout) + `tp.Shutdown`
  on exit so the last spans flush.
- Binary size: OTel SDK ~ +1.1 MB stripped (plan predicted ~+0.91 MB).
- `MCPSIM_OTLP_ENDPOINT` is accepted (and validated as a plain string)
  but no OTLP exporter is wired; JSONL only, per plan.
