# Open MCP Control Plane

An open-source, self-hosted control plane for discovering, evaluating, deploying, securing, and operating MCP servers and agent skills through one stable gateway.

> **Status:** v0.1 MVP backbone. APIs may still change before the first tagged release.

## Vision

**Discover → assess → approve → deploy → connect → observe → update or roll back**

## What works now

- Go control-plane service with SQLite persistence
- Official MCP Registry adapter + static bootstrap catalog
- Deployment plans with default-deny policy findings
- Explicit approvals for high-risk plans
- Runtime abstraction with fake runtime (tests/dev) and Docker CLI adapter
- Profiles, hashed gateway clients, and authenticated gateway tool routing
- Admin bearer auth, CSRF helpers, readiness/health endpoints, audit events
- Embedded operator dashboard
- `mcpctl` CLI
- Management MCP endpoint (`POST /mcp/management`)
- AES-GCM encrypted secret references (`OPENMCP_SECRETS_MASTER_KEY`)
- Streamable HTTP JSON-RPC gateway (`initialize`, `tools/list`, `tools/call`)
- Installation update/rollback with runtime ref snapshots
- Heuristic image scanning + package risk scoring (A–F grades)
- Skills package model with profile bindings
- ToolHive catalog federation source (seed + remote, degrade-safe)

## Quick start

```bash
export GOTOOLCHAIN=local
go test ./...
go build -o bin/controlplane ./cmd/controlplane
go build -o bin/mcpctl ./cmd/mcpctl

OPENMCP_USE_FAKE_RUNTIME=true ./bin/controlplane
```

On first boot, if `OPENMCP_BOOTSTRAP_ADMIN_TOKEN` is unset, a one-time admin token is printed to logs. Store it securely.

```bash
export OPENMCP_URL=http://127.0.0.1:8080
export OPENMCP_TOKEN='your-admin-token'
./bin/mcpctl status
./bin/mcpctl search postgres
```

## Docker Compose

```bash
docker compose up --build
```

API/GUI: `http://127.0.0.1:8080`

## Core API surface

- `GET /healthz`, `GET /readyz`, `GET /v1/info`
- `GET /v1/catalog/search`, `GET /v1/catalog/packages/{id}`, `GET /v1/catalog/sources/status`
- Admin: `/v1/admin/plans`, `/approvals`, `/installations` (+ update/rollback), `/profiles`, `/clients`, `/secrets`, `/skills`, `/skill-bindings`, `/scan/image`, `/audit`, `/gateway/status`
- Risk: `GET /v1/catalog/packages/{id}/risk`, `POST /v1/catalog/scan-image`
- Gateway (REST helpers): `GET /gateway/tools`, `POST /gateway/invoke`
- Gateway (Streamable HTTP / JSON-RPC): `POST /gateway/mcp`, `GET /gateway/mcp`
- Profile MCP aliases: `POST /mcp/profiles/{name}`, `GET /mcp/profiles/{name}`
- Management MCP: `POST /mcp/management`

## Secrets

Set `OPENMCP_SECRETS_MASTER_KEY` to encrypt secret references at rest (AES-GCM). List/create APIs never return plaintext values.


## Authentication & secrets

Admin API auth accepts any of:
- Bootstrap admin bearer token (`OPENMCP_BOOTSTRAP_ADMIN_TOKEN`)
- OIDC browser login (`/v1/auth/oidc/login`) minting an `openmcp_admin_session` cookie
- Bearer OIDC ID token or signed session token

OIDC env vars (optional):
```bash
OPENMCP_OIDC_ENABLED=true
OPENMCP_OIDC_ISSUER_URL=https://accounts.example.com
OPENMCP_OIDC_CLIENT_ID=...
OPENMCP_OIDC_CLIENT_SECRET=...
OPENMCP_OIDC_REDIRECT_URL=https://cp.example.com/v1/auth/oidc/callback
OPENMCP_OIDC_ALLOWED_EMAILS=you@example.com
```

Secrets backends (`OPENMCP_SECRETS_BACKEND`):
- `local` (default) — AES-GCM with `OPENMCP_SECRETS_MASTER_KEY`
- `env` — process/env injected values (`OPENMCP_SECRET_<ID>`)
- `file` — files under `OPENMCP_SECRETS_FILE_DIR`

Gateway sessions:
- `POST /gateway/mcp` initialize returns `Mcp-Session-Id`
- `GET /gateway/mcp` with `Accept: text/event-stream` opens SSE
- `DELETE /gateway/mcp` with `Mcp-Session-Id` ends the session

## Security notes

- Secrets and tokens are never returned by list/read endpoints
- High-risk plans require approval before apply
- Containers default to non-root / dropped caps / resource limits
- Profile allowlists control which tools clients can invoke

See [docs/security.md](docs/security.md) and [SECURITY.md](SECURITY.md).

## Roadmap

1. ~~Full Streamable HTTP MCP transport parity~~ (v0.1: initialize/tools/list/tools/call + sessions)
2. ~~OIDC identity and production secret backends~~ (local AES / env / file backends; OIDC login + session tokens)
3. ~~Production GUI polish~~ (sectioned operator UI: marketplace, ops, gateway/sessions, skills, secrets)
4. ~~Update/rollback workflows + heuristic image scanning / package risk scoring~~
5. ~~Skills package model and ToolHive catalog federation~~
6. ~~SSE streaming responses and full session lifecycle for long-running tools~~

### Optional next
- Real Docker runtime on production VPS + Compose installer polish
- PostgreSQL production store
- Official MCP Registry deeper federation
- OpenTelemetry metrics/traces

## License

Apache License 2.0.
