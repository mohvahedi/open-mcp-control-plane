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
- Admin: `/v1/admin/plans`, `/approvals`, `/installations` (+ update/rollback), `/profiles`, `/clients`, `/secrets`, `/scan/image`, `/audit`, `/gateway/status`
- Risk: `GET /v1/catalog/packages/{id}/risk`, `POST /v1/catalog/scan-image`
- Gateway (REST helpers): `GET /gateway/tools`, `POST /gateway/invoke`
- Gateway (Streamable HTTP / JSON-RPC): `POST /gateway/mcp`, `GET /gateway/mcp`
- Profile MCP aliases: `POST /mcp/profiles/{name}`, `GET /mcp/profiles/{name}`
- Management MCP: `POST /mcp/management`

## Secrets

Set `OPENMCP_SECRETS_MASTER_KEY` to encrypt secret references at rest (AES-GCM). List/create APIs never return plaintext values.

## Security notes

- Secrets and tokens are never returned by list/read endpoints
- High-risk plans require approval before apply
- Containers default to non-root / dropped caps / resource limits
- Profile allowlists control which tools clients can invoke

See [docs/security.md](docs/security.md) and [SECURITY.md](SECURITY.md).

## Roadmap

1. ~~Full Streamable HTTP MCP transport parity~~ (v0.1 partial: initialize/tools/list/tools/call)
2. OIDC identity and production secret backends
3. Production GUI polish (risk scoring APIs shipped)
4. ~~Update/rollback workflows + heuristic image scanning / package risk scoring~~
5. Skills package model and ToolHive catalog federation
6. SSE streaming responses and full session lifecycle for long-running tools

## License

Apache License 2.0.
