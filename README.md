# Open MCP Control Plane

**Open-source, self-hosted control plane for discovering, evaluating, deploying, securing, and operating [MCP](https://modelcontextprotocol.io) servers and agent skills — through one stable gateway.**

[![CI](https://github.com/mohvahedi/open-mcp-control-plane/actions/workflows/ci.yml/badge.svg)](https://github.com/mohvahedi/open-mcp-control-plane/actions/workflows/ci.yml)
[![License](https://img.shields.io/badge/License-Apache_2.0-blue.svg)](LICENSE)

> **Status:** v0.1 MVP. Suitable for local development, self-hosting experiments, and contributions. Public APIs may still change before the first tagged release.

---

## Why this exists

MCP servers are proliferating across registries, GitHub repos, container images, and remote endpoints. Operators end up with:

- Many ad-hoc installs and credentials
- No shared approval or risk review
- Clients that each need a separate connection per server
- Skills and prompts mixed in without clear trust boundaries

**Open MCP Control Plane** turns that chaos into one workflow:

```text
Discover → Assess → Approve → Deploy → Connect → Observe → Update / Roll back
```

You (or an agent) search catalogs, inspect risk, create a deployment plan, get human approval when needed, apply the install, attach tools to a **profile**, and expose only allowlisted tools through **one authenticated gateway** that standard MCP clients can use (including Notion and other Streamable HTTP clients).

---

## Features (v0.1)

| Area | What you get |
|------|----------------|
| **Catalog** | Official MCP Registry adapter, ToolHive federation (degrade-safe), static bootstrap packages |
| **Risk** | Heuristic image scanning, package risk scores (A–F grades) |
| **Lifecycle** | Plans → approvals → apply; start/stop/restart/disable/uninstall; **update** and **rollback** |
| **Runtime** | Docker CLI adapter + fake runtime for tests/dev |
| **Gateway** | REST helpers + **Streamable HTTP** JSON-RPC (`initialize`, `tools/list`, `tools/call`) |
| **Sessions / SSE** | `Mcp-Session-Id` lifecycle, SSE streams, progress notifications |
| **Profiles** | Curated installation sets + tool allowlists per client use case |
| **Skills** | Versioned skill packages with profile bindings |
| **Secrets** | Pluggable backends: local AES-GCM, env, file (never returned in list APIs) |
| **Auth** | Bootstrap admin token, optional **OIDC** (PKCE + session cookies/tokens) |
| **Interfaces** | Embedded operator **GUI**, **`mcpctl` CLI**, management **MCP** endpoint |
| **Ops** | Health/ready, audit events, CSRF helpers, SQLite persistence |

---

## Architecture

```text
  Web GUI ────────┐
  mcpctl CLI ─────┼──►  Versioned Control Plane API  (/v1/...)
  Agent MCP ──────┘              │
                                 ├── Catalog & search
                                 ├── Policy & approvals
                                 ├── Deployment controller
                                 ├── Secrets & audit
                                 └── Skills & profiles
                                          │
                                   Docker / remote runtime
                                          │
                                   Installed MCP servers
                                          │
                              Unified authenticated gateway
                           /gateway/mcp  ·  /mcp/profiles/{name}
                                          │
                              MCP clients (Notion, IDEs, agents)
```

**Design principles**

1. **Open-source first** — Apache-2.0, public roadmap, transparent decisions  
2. **Secure by default** — untrusted until evaluated; high-risk plans need approval  
3. **One VPS should be enough** — Docker-friendly path; portable internals  
4. **Standards over lock-in** — MCP Streamable HTTP, registry APIs, OCI images  
5. **Human control at trust boundaries** — credentials, mounts, public exposure  
6. **One stable client connection** — profiles + gateway, not N server URLs  
7. **Composable upstreams** — registry, runtime, secrets, and identity stay swappable  

More detail: [docs/architecture.md](docs/architecture.md) · [docs/security.md](docs/security.md)

---

## Quick start (local)

**Requirements:** Go 1.22+, optional Docker for Compose / real runtime.

```bash
git clone https://github.com/mohvahedi/open-mcp-control-plane.git
cd open-mcp-control-plane

# Tests & binaries
make test
make build
# → bin/controlplane , bin/mcpctl

# Dev server (fake runtime — no Docker required)
export OPENMCP_USE_FAKE_RUNTIME=true
export OPENMCP_SECRETS_MASTER_KEY="$(openssl rand -hex 32)"
./bin/controlplane
```

On first boot, if `OPENMCP_BOOTSTRAP_ADMIN_TOKEN` is unset, a **one-time admin token is printed to the process log**. Store it securely.

```bash
export OPENMCP_URL=http://127.0.0.1:8080
export OPENMCP_TOKEN='paste-admin-token-here'

./bin/mcpctl status
./bin/mcpctl search postgres
./bin/mcpctl gateway status
```

Open the operator UI: **[http://127.0.0.1:8080/](http://127.0.0.1:8080/)**  
Paste the admin token in the header (session-only storage in the browser).

---

## Docker Compose

```bash
docker compose up --build
```

- API + GUI: `http://127.0.0.1:8080`
- Data volume: `openmcp-data`
- Default Compose profile uses `OPENMCP_USE_FAKE_RUNTIME=true` and hardened container defaults (`read_only`, `cap_drop: ALL`, memory/PID limits)

Copy [`.env.example`](.env.example) as a starting point for real secrets and tokens — **never commit real credentials**.

---

## Operator walkthrough

Typical first installation (CLI):

```bash
# 1) Discover
./bin/mcpctl search filesystem
./bin/mcpctl inspect example-mcp-server
./bin/mcpctl risk example-mcp-server
./bin/mcpctl scan ghcr.io/example/mcp:latest   # heuristic image scan

# 2) Plan (JSON body — remote endpoint example for local mocks)
./bin/mcpctl plan create '{
  "name": "demo",
  "remote_endpoint": "http://127.0.0.1:9001",
  "transport": "streamable-http"
}'

# 3) Approve if policy marks the plan high-risk
./bin/mcpctl approval create <plan-id> "Reviewed for lab use"

# 4) Apply
./bin/mcpctl install apply <plan-id>
./bin/mcpctl list

# 5) Profile + client gateway token
./bin/mcpctl profile create notion
./bin/mcpctl profile add <profile-id> <installation-id>
./bin/mcpctl profile tools <profile-id> <installation-id>.some_tool
./bin/mcpctl client create <profile-id> notion-client
# → save client_id.token  (shown once)

# 6) Point an MCP client at Streamable HTTP
#    URL:  http://127.0.0.1:8080/gateway/mcp
#    Auth: Bearer <clientId>.<token>
```

**Skills**

```bash
./bin/mcpctl skill list
./bin/mcpctl skill create '{"name":"Safe ops","kind":"prompt","content":"# Prefer read-only tools"}'
./bin/mcpctl skill bind <profile-id> <skill-id>
```

**Lifecycle**

```bash
./bin/mcpctl install health <id>
./bin/mcpctl install logs <id>
./bin/mcpctl update <id> [image]
./bin/mcpctl rollback <id>
./bin/mcpctl install stop <id>
```

---

## Interfaces

### 1. Web GUI

Embedded single-page operator UI at `/`:

- Marketplace search & image risk scan  
- Deployment plans, approvals, installations (start/stop/update/rollback)  
- Profiles, gateway clients, live sessions  
- Skills & bindings  
- Secrets (write-only values)  
- Optional **OIDC login** when enabled  

### 2. CLI (`mcpctl`)

```text
mcpctl status | search | inspect | risk | scan
mcpctl plan create|show|list
mcpctl approval create
mcpctl install apply|list|health|logs|start|stop|restart|disable|uninstall
mcpctl update | rollback
mcpctl profile create|list|add|tools
mcpctl client create|list|revoke
mcpctl skill list|create|show|delete|bind|bindings|unbind
mcpctl gateway status
```

Environment:

| Variable | Default | Purpose |
|----------|---------|---------|
| `OPENMCP_URL` | `http://127.0.0.1:8080` | Control plane base URL |
| `OPENMCP_TOKEN` | _(empty)_ | Admin bearer token |

Add `--json` for machine-readable output.

### 3. Management MCP

Agents can call a small, stable tool set (admin-authenticated):

```http
POST /mcp/management
Authorization: Bearer <admin-token>
```

Includes catalog/lifecycle-oriented management tools (e.g. search, plans, installations, skills) enforced by the same policy layer as the HTTP admin API — **not** unrestricted shell access.

### 4. Client gateway

| Transport | Endpoint | Auth |
|-----------|----------|------|
| REST tools list | `GET /gateway/tools` | `Bearer clientId.token` |
| REST invoke | `POST /gateway/invoke` | same |
| Streamable HTTP JSON-RPC | `POST /gateway/mcp` | same |
| Capability / session probe | `GET /gateway/mcp` | same |
| SSE stream | `GET /gateway/mcp` + `Accept: text/event-stream` | same |
| End session | `DELETE /gateway/mcp` + `Mcp-Session-Id` | same |
| Profile alias | `/mcp/profiles/{name}` (GET/POST/DELETE) | same |

**JSON-RPC methods:** `initialize`, `notifications/initialized`, `ping`, `tools/list`, `tools/call`  
**Protocol version advertised:** `2024-11-05`  
**Tool names:** namespaced as `<installation-id>.<tool-name>`, filtered by profile allowlist.

Session flow:

1. `initialize` → response header `Mcp-Session-Id`  
2. Subsequent calls may send `Mcp-Session-Id`  
3. Optional SSE subscription receives messages / progress events  
4. `DELETE` closes the session  

---

## HTTP API map

### Public / unauthenticated

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/` | Operator GUI |
| `GET` | `/healthz` | Liveness |
| `GET` | `/readyz` | Readiness (DB) |
| `GET` | `/v1/info` | Version & feature flags |
| `GET` | `/v1/csrf` | CSRF cookie + token |
| `GET` | `/v1/auth/status` | Auth / OIDC / secrets backend status |
| `GET` | `/v1/auth/oidc/login` | Start OIDC (if enabled) |
| `GET` | `/v1/auth/oidc/callback` | OIDC callback |
| `POST` | `/v1/auth/logout` | Clear session cookie |
| `GET` | `/v1/catalog/search` | Search packages |
| `GET` | `/v1/catalog/packages/{id}` | Package detail |
| `GET` | `/v1/catalog/packages/{id}/risk` | Risk grade |
| `GET` | `/v1/catalog/sources/status` | Catalog source health |
| `POST` | `/v1/catalog/scan-image` | Heuristic image scan |
| `GET` | `/v1/skills` | Public skill list (metadata) |

### Admin (`Authorization: Bearer <admin>` or OIDC session)

| Area | Paths |
|------|--------|
| Plans | `/v1/admin/plans`, `/v1/admin/plans/{id}` |
| Approvals | `/v1/admin/approvals` |
| Installations | `/v1/admin/installations`, `.../apply`, `.../{id}/{health,logs,start,stop,restart,disable,uninstall,update,rollback}` |
| Profiles | `/v1/admin/profiles`, `.../{id}/installations`, `.../{id}/tools` |
| Clients | `/v1/admin/clients`, `.../{id}/revoke` |
| Secrets | `/v1/admin/secrets`, `/v1/admin/secrets/backend` |
| Skills | `/v1/admin/skills`, `/v1/admin/skill-bindings` |
| Gateway ops | `/v1/admin/gateway/status`, `/v1/admin/sessions` |
| Audit | `/v1/admin/audit` |
| Scan | `/v1/admin/scan/image` |
| Management MCP | `POST /mcp/management` |

Browser mutations from the GUI should send CSRF headers when an `Origin` is present (`X-CSRF-Token` matching the `openmcp_csrf` cookie).

---

## Authentication

Admin APIs accept **any one of**:

1. **Bootstrap admin bearer token** — hashed at rest; set via `OPENMCP_BOOTSTRAP_ADMIN_TOKEN` or generated once at first start  
2. **OIDC browser login** — `/v1/auth/oidc/login` → IdP → callback mints `openmcp_admin_session` cookie  
3. **Bearer session token** or raw **OIDC ID token** (API clients)  

### Enable OIDC

```bash
export OPENMCP_OIDC_ENABLED=true
export OPENMCP_OIDC_ISSUER_URL=https://accounts.example.com
export OPENMCP_OIDC_CLIENT_ID=openmcp
export OPENMCP_OIDC_CLIENT_SECRET=...
export OPENMCP_OIDC_REDIRECT_URL=https://cp.example.com/v1/auth/oidc/callback
export OPENMCP_OIDC_ALLOWED_EMAILS=you@example.com
# optional:
# OPENMCP_OIDC_ALLOWED_SUBJECTS=sub-1,sub-2
# OPENMCP_OIDC_SCOPES=openid,profile,email
# OPENMCP_OIDC_SESSION_TTL=8h
# OPENMCP_SESSION_HMAC_SECRET=...   # defaults to secrets master key
```

Gateway clients use **separate** credentials: `Bearer <clientId>.<rawToken>` (token shown once at creation; stored hashed).

---

## Secrets

Secrets are stored as **references**. List/read APIs never return plaintext.

| `OPENMCP_SECRETS_BACKEND` | Behavior |
|---------------------------|----------|
| `local` (default) | AES-GCM ciphertext in SQLite; key from `OPENMCP_SECRETS_MASTER_KEY` |
| `env` | Values keyed as `OPENMCP_SECRET_<ID>` (in-process + environment) |
| `file` | One file per secret under `OPENMCP_SECRETS_FILE_DIR` (mode `0600`) |

```bash
export OPENMCP_SECRETS_MASTER_KEY="$(openssl rand -hex 32)"   # required / strongly recommended
export OPENMCP_SECRETS_BACKEND=local
# export OPENMCP_SECRETS_BACKEND=file
# export OPENMCP_SECRETS_FILE_DIR=/var/lib/openmcp/secrets
```

---

## Configuration reference

| Variable | Default | Description |
|----------|---------|-------------|
| `OPENMCP_HOST` | `0.0.0.0` | Bind address |
| `OPENMCP_PORT` | `8080` | Bind port |
| `OPENMCP_VERSION` | `dev` | Reported version string |
| `OPENMCP_DB_PATH` | `file:openmcp.db?_pragma=busy_timeout(5000)` | SQLite DSN |
| `OPENMCP_BOOTSTRAP_ADMIN_TOKEN` | _(empty)_ | Fixed admin token; else generated once |
| `OPENMCP_REQUEST_TIMEOUT` | `10s` | Outbound HTTP timeout |
| `OPENMCP_REGISTRY_BASE_URL` | `https://registry.modelcontextprotocol.io` | Official registry |
| `OPENMCP_REGISTRY_TIMEOUT` | `5s` | Registry HTTP timeout |
| `OPENMCP_REGISTRY_PAGE_LIMIT` | `25` | Registry page size (1–100) |
| `OPENMCP_USE_FAKE_RUNTIME` | `false` | Skip Docker; in-memory fake runtime |
| `OPENMCP_DOCKER_BINARY` | `docker` | Docker CLI path |
| `OPENMCP_GATEWAY_PATH` | `/gateway` | Gateway path prefix (informational) |
| `OPENMCP_SECRETS_MASTER_KEY` | `dev-only-change-me` | AES key material (change in prod) |
| `OPENMCP_SECRETS_BACKEND` | `local` | `local` \| `env` \| `file` |
| `OPENMCP_SECRETS_FILE_DIR` | `./secrets-data` | File backend directory |
| `OPENMCP_MCP_SESSION_TTL` | `30m` | Gateway session idle TTL |
| `OPENMCP_OIDC_*` | — | See [Authentication](#authentication) |

---

## Security model (summary)

- Registry metadata, packages, containers, skills, and remote endpoints are **untrusted inputs**  
- High-risk plans (privileged, host network/mounts, public exposure, …) require **explicit approval** before apply  
- Admin and gateway credentials stored as **one-way hashes**  
- Secret **values never leave** list/read APIs  
- Managed containers default to **non-root**, dropped caps, no-new-privileges, resource limits  
- Downstream tools are **default-deny** until added to a profile allowlist  
- Audit events redact sensitive metadata  

Report vulnerabilities via GitHub private vulnerability reporting — see [SECURITY.md](SECURITY.md).

---

## Repository layout

```text
cmd/
  controlplane/     # main HTTP service
  mcpctl/           # operator CLI
internal/
  catalog/          # registry + ToolHive adapters
  config/           # env configuration
  domain/           # core models
  policy/           # risk / approval policy
  runtime/          # Docker + fake runtimes
  scanner/          # heuristic image/package risk
  security/         # tokens, AES secrets, OIDC, backends
  server/           # HTTP API, gateway, GUI, sessions
  store/            # SQLite repository
docs/               # architecture & security notes
compose.yaml        # single-node Compose stack
Dockerfile
```

---

## Development

```bash
make fmt          # gofmt
make vet
make test
make race         # go test -race ./...
make build
make run          # fake runtime via go run
```

CI (`.github/workflows/ci.yml`) runs `gofmt`, `go vet`, `go test`, `go test -race`, and builds both binaries on every PR.

Contribution guidelines: [CONTRIBUTING.md](CONTRIBUTING.md) · Code of conduct: [CODE_OF_CONDUCT.md](CODE_OF_CONDUCT.md)

---

## Roadmap

### Done in v0.1

- [x] Control plane API + SQLite + CLI + embedded GUI  
- [x] Catalog search (official registry + ToolHive federation)  
- [x] Plans, approvals, install lifecycle, update/rollback  
- [x] Profiles, gateway clients, Streamable HTTP + SSE sessions  
- [x] Skills model + bindings  
- [x] Heuristic scanning / risk grades  
- [x] Encrypted secret references + pluggable backends  
- [x] Optional OIDC admin identity  

### Next (post–v0.1)

- [ ] Production Docker runtime defaults on a real VPS + installer polish  
- [ ] Deeper Official MCP Registry coverage and provenance UX  
- [ ] PostgreSQL production store option  
- [ ] OpenTelemetry metrics and traces  
- [ ] stdio → authenticated remote bridge hardening  
- [ ] Signed release artifacts and Helm chart  

---

## License

Licensed under the [Apache License 2.0](LICENSE).

---

## Acknowledgments

Built for operators and agent builders who want **one trustworthy place** to run MCP — without giving every model a blank check on your infrastructure.

**Project:** [github.com/mohvahedi/open-mcp-control-plane](https://github.com/mohvahedi/open-mcp-control-plane)
