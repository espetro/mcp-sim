# Authentication

mcp-sim protects its HTTP surface with a single static bearer token. The stdio
transport stays auth free (see below). This page covers the auth model, token
resolution, first boot behavior, client configuration for Claude Code, Claude
Desktop, Cursor, and coding agents, the insecure-no-auth gate, and rotation.

## Auth model

- Only the streamable HTTP transport (`/mcp`) enforces bearer auth. Requests
  without a valid `Authorization: Bearer <token>` header get `401` with a
  spec-shaped `WWW-Authenticate: Bearer` challenge.
- `/healthz` is exempt, so load balancers and service managers can probe
  without the token.
- `/.well-known/oauth-protected-resource` (RFC 9728 protected resource
  metadata) is served unauthenticated so spec-aware clients can discover the
  Bearer requirement. It advertises `bearer_methods_supported: ["header"]` and
  never advertises authorization servers: mcp-sim is a static-token resource,
  never an OAuth authorization server.
- Token comparison is constant time (`crypto/subtle`), and verified sessions
  are bound per user by the MCP SDK, preventing cross-user session hijack.
- The stdio transport (`mcp-sim mcp`) is auth free by design: the client
  spawns the process itself, so trust is inherited from process ownership.
  Passing auth flags or token config to stdio only triggers a warning; the
  config is ignored.

## Token resolution precedence

The token is resolved in this order, first non-empty wins:

1. `MCPSIM_AUTH_TOKEN` environment variable
2. `server.auth.token` in `config.yaml`
3. First boot auto-generated token, persisted to disk (see below)

The chosen source is logged at startup (never the raw token).

## First boot behavior

When neither env nor config provides a token, the server generates one: 32
bytes from `crypto/rand`, base64url encoded. On first boot it:

- Saves the token to `~/.config/mcp-sim/token` with `0600` permissions.
- Writes a ready-to-paste client config next to it,
  `~/.config/mcp-sim/client-snippet.json` (also `0600`, it embeds the token).
- Logs the file paths only, never the token itself, in non-interactive
  (service, pipe, CI) contexts.
- On an interactive TTY, prints the token and the client snippet to stderr
  once, so a human can copy it immediately.

On every later start the existing token file is reused as-is. The server
never rotates the token silently on restart.

## Client configuration

### Claude Code

```bash
claude mcp add --transport http mcp-sim http://host:9090/mcp --header "Authorization: Bearer <token>"
```

For a remote host over ssh, fetch the token in one line:

```bash
claude mcp add --transport http mcp-sim https://host:9090/mcp --header "Authorization: Bearer $(ssh host cat ~/.config/mcp-sim/token)"
```

### Claude Desktop and Cursor

Both use the `mcpServers` JSON shape. The generated
`~/.config/mcp-sim/client-snippet.json` is exactly this, ready to paste:

```json
{
  "mcpServers": {
    "mcp-sim": {
      "url": "http://localhost:9090/mcp",
      "headers": {
        "Authorization": "Bearer <token>"
      }
    }
  }
}
```

### Coding agents

Agents should not scrape logs. After first boot, retrieve the snippet
non-interactively:

```bash
mcp-sim auth print-snippet
```

It prints the `mcpServers` JSON block to stdout.

## Environment variables

| Env var | Meaning |
|---|---|
| `MCPSIM_AUTH_TOKEN` | Bearer token for `/mcp`; takes precedence over `server.auth.token` and the persisted token file |
| `MCPSIM_INSECURE_NO_AUTH` | Set to a truthy value to disable bearer auth (same as `--insecure-no-auth`; still gated, see below) |
| `MCPSIM_TRUSTED_NETWORK` | Set to `true` to acknowledge the network is trusted; also satisfies the non-loopback gate for `--insecure-no-auth` |

## Disabling auth: insecure-no-auth gate

`--insecure-no-auth` (or `MCPSIM_INSECURE_NO_AUTH`) turns off bearer auth.
This is gated: it is refused when the listen address is not loopback, unless
you pass a second acknowledgment, either `--insecure-no-auth-ack` or
`MCPSIM_TRUSTED_NETWORK=true`.

Warning: loopback means loopback only. `127.0.0.1:9090` and `localhost:9090`
qualify. `:9090` and `0.0.0.0:9090` bind to all interfaces and are NOT
loopback, so they are refused without the ack. Typical legitimate uses of the
gate:

- Pure loopback servers behind a local reverse proxy or gateway that does its
  own authentication: run `--insecure-no-auth --insecure-no-auth-ack`.
- Tailnet or VPN deployments where network ACLs are the intended only
  boundary: run with `MCPSIM_TRUSTED_NETWORK=true`.

## 401 behavior

A request to `/mcp` without or with a wrong token returns `401 Unauthorized`
with a `WWW-Authenticate: Bearer` header (including the resource metadata URL
when applicable). Spec-aware clients use this plus the RFC 9728 document to
configure themselves; plain HTTP clients must attach the header.

## Rotation policy

The token never rotates silently. If you need to rotate:

1. Delete `~/.config/mcp-sim/token` (or change `server.auth.token` / the env
   var).
2. Restart mcp-sim. A new token is generated and a fresh
   `client-snippet.json` is written.
3. Update every client. Old clients keep sending the old token and receive
   `401` until updated.

## Commercial deployment notes

- Behind a trusted ingress or API gateway that terminates authentication
  (mTLS, SSO, JWT verification): it is reasonable to run mcp-sim with auth
  disabled and the acknowledgment flag, keeping authentication at the edge.
  Bind to loopback or a private interface so only the gateway can reach it.
- On a tailnet (Tailscale or similar): keep auth enabled as defense in depth
  over the private network. See [Tailscale setup](tailscale.md) for the full
  remote configuration, including the ssh token-fetch one-liner.
