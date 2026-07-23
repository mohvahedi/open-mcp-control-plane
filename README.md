# Open MCP Control Plane

An open-source, self-hosted control plane for discovering, evaluating, deploying, securing, and operating MCP servers and agent skills through one stable gateway.

> **Status:** Early foundation. The APIs and repository structure will change before the first tagged release.

## Vision

Turn the fragmented MCP ecosystem into one coherent workflow:

**Discover → assess → approve → deploy → connect → observe → update or roll back**

The project is designed to run on a single VPS with Docker and grow into a team or Kubernetes deployment without locking users into a proprietary registry, runtime, or gateway.

## Interfaces

- A responsive web GUI for discovery, approvals, profiles, health, and logs
- A built-in CLI for administrators and automation
- A stable management MCP endpoint for agents
- A unified Streamable HTTP gateway for downstream MCP servers

## Current foundation

The first branch establishes:

- A dependency-free Go control-plane service
- Health and version endpoints
- A catalog source abstraction with normalized package metadata
- Search and deduplication behavior
- Unit tests
- A minimal non-root container image
- Docker Compose and CI foundations
- Initial architecture and security documentation

## Run locally

Requires Go 1.22 or newer.

```bash
go test ./...
go run ./cmd/controlplane
curl http://localhost:8080/healthz
curl 'http://localhost:8080/v1/catalog/search?q=postgres'
```

## Run with Docker

```bash
docker compose up --build
```

The API listens on `http://localhost:8080` by default.

## Roadmap

1. Gateway feasibility and real-client compatibility
2. Official MCP Registry and ToolHive catalog adapters
3. Deployment plans, policy checks, and Docker lifecycle management
4. Profiles and unified gateway routing
5. CLI and web GUI
6. Agent-facing management MCP
7. Supply-chain security, observability, updates, and rollback

## Contributing

The project is at the architecture and feasibility stage. See [CONTRIBUTING.md](CONTRIBUTING.md), [docs/architecture.md](docs/architecture.md), and [docs/security.md](docs/security.md).

## License

Apache License 2.0.
