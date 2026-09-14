# Observability: OTel spans and the JSONL audit log

mcp-sim uses OpenTelemetry spans as the observability spine. Every MCP
request gets a span; every `tools/call` gets a span named
`mcp_sim.<tool>` carrying a standard audit attribute set. Spans are
written as JSON Lines (one JSON object per line) to a local audit file
that a CI runner can scrape, ship, or post-process with `jq`.

Observability is **off by default**. When disabled, no tracer provider is
installed, no spans are recorded, and the audit file is never created.

## Enabling

```bash
# Enable tracing + audit log (default location)
MCPSIM_OBSERVABILITY_ENABLED=true mcp-sim serve

# Equivalent YAML (config.yaml)
observability:
  enabled: true
  audit_path: file
```

| Config key | Env var | Default | Meaning |
|---|---|---|---|
| `observability.enabled` | `MCPSIM_OBSERVABILITY_ENABLED` | `false` | Enable the OTel tracer provider and audit export |
| `observability.audit_path` | `MCPSIM_AUDIT_LOG` | `file` | `stdout`, `file`, or an explicit path |
| `observability.otlp_endpoint` | `MCPSIM_OTLP_ENDPOINT` | (unset) | Reserved for a future OTLP/HTTP exporter; accepted but not yet used |

`MCPSIM_AUDIT_LOG` accepts three shapes:

- `file` (default): `~/.config/mcp-sim/audit.jsonl`
- `stdout`: JSONL lines interleave with the human slog stream; filter
  with `jq -c 'select(.schema=="mcp-sim.audit.v1")'`
- an absolute or relative path: `/var/log/mcp-sim/audit.jsonl`

File sinks rotate at 50 MB with 5 generations kept (`.1` through `.5`
suffixes).

## JSONL schema

Every line carries the forward-compat header `schema: mcp-sim.audit.v1`.
Attributes named `audit.*` are promoted to a top-level `audit` object.

```json
{
  "schema": "mcp-sim.audit.v1",
  "name": "mcp_sim.install_app",
  "start_time": "2026-09-14T11:24:18.512Z",
  "end_time": "2026-09-14T11:24:18.984Z",
  "trace_id": "a1b2c3d4e5f60718293a4b5c6d7e8f90",
  "span_id": "3f2a9b1c4d5e6f70",
  "parent_span_id": "1122334455667788",
  "status": "ok",
  "attributes": {
    "tool.name": "install_app",
    "tool.caller_ip": "10.0.0.4",
    "audit.outcome": "ok",
    "audit.latency_ms": "472",
    "audit.params_sha256": "8a3f1c2d9b7e4a01"
  },
  "audit": {
    "audit.outcome": "ok",
    "audit.latency_ms": "472",
    "audit.params_sha256": "8a3f1c2d9b7e4a01"
  }
}
```

| Key | When set |
|---|---|
| `name` | Every span: `mcp.<method>` for requests, `mcp_sim.<tool>` for tool calls |
| `tool.name` | Tool-call spans |
| `tool.caller_ip` | HTTP transport spans (best-effort: `X-Forwarded-For` then remote addr) |
| `audit.params_sha256` | Tool-call spans; first 16 hex chars of the SHA-256 of the arguments JSON |
| `audit.outcome` | Tool-call spans: `ok` or `error` |
| `audit.latency_ms` | Tool-call spans |
| `audit.error_code` | Failed tool calls; carries the error text |
| `status` | Span status: `unset`, `ok`, or `error: <description>` |

Raw tool arguments are **never** written to the audit log. Only their
hash is. Grep check: `grep -c arguments audit.jsonl` returns 0.

## Trace propagation

mcp-sim accepts W3C `traceparent` headers on `/mcp` POSTs. A runner that
starts a trace sees mcp-sim's spans parented under its own:

```bash
curl -H "traceparent: 00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01" ...
```

Cross-process propagation into child processes (for example the
agent-device backend behind the verifier tools) is v1.1 scope; their work
shows up in the audit log as the latency of the driving tool call.

## jq recipes

```bash
# Outcome histogram
jq -r '.audit["audit.outcome"]' ~/.config/mcp-sim/audit.jsonl | sort | uniq -c

# Slowest tool calls
jq -r 'select(.name | startswith("mcp_sim.")) | [.audit["audit.latency_ms"], .name] | @tsv' \
  ~/.config/mcp-sim/audit.jsonl | sort -rn | head

# Schema sanity (should print one value)
jq -r '.schema' ~/.config/mcp-sim/audit.jsonl | sort -u

# Errors only
jq -c 'select(.audit["audit.outcome"]=="error")' ~/.config/mcp-sim/audit.jsonl
```

## Limitations

- Spans only. Metrics and log export are not wired yet.
- JSONL exporter only. The OTel SDK seam makes an OTLP/HTTP exporter a
  small follow-up (`observability.otlp_endpoint` is already reserved).
- No sampling: every span is recorded.
