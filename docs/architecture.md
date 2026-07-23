# Architecture

## Overview

Open MCP Control Plane separates human and agent interfaces from policy, deployment, and gateway concerns.

```text
Web GUI ────────┐
CLI ────────────┼──> Versioned Control Plane API
Agent MCP ──────┘       │
                         ├── Registry and search
                         ├── Policy and approvals
                         ├── Deployment controller
                         ├── Secrets and audit
                         └── Health and operations
                                  │
                             Docker/OCI runtime
                                  │
                         Installed MCP servers
                                  │
                         Unified MCP gateway
                                  │
                           MCP client profiles
```

## Boundaries

- **Catalog adapters** normalize upstream registries while retaining provenance.
- **Policy** decides what may be installed, executed, and exposed.
- **Deployment adapters** manage artifacts without leaking Docker-specific concepts into the domain model.
- **Profiles** define which tools a client can access.
- **Gateway adapters** expose approved downstream tools through standard MCP transports.
- **Interfaces** share one API and never reimplement authorization or policy logic.

## Initial API

- `GET /healthz` — process health
- `GET /v1/info` — build and maturity information
- `GET /v1/catalog/search?q=` — normalized catalog search

The initial service intentionally has no external runtime dependencies. Durable storage, authentication, real catalog sources, and deployment operations will be introduced behind interfaces.

## Decision: monorepo

The API, CLI, web application, deployment manifests, and specifications will remain in one repository until independent release cadences are justified.
