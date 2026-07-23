# Contributing

Thank you for helping build an open, safe control plane for MCP servers and agent skills.

## Development

```bash
gofmt -w $(find . -name '*.go')
go vet ./...
go test -race ./...
go build ./cmd/controlplane
```

## Pull requests

- Keep changes focused and explain the user or operator outcome.
- Add tests for behavior changes.
- Document security and compatibility implications.
- Never commit credentials, tokens, production endpoints, or personal data.
- Use conventional commit-style subjects where practical.

## Architecture changes

Changes to trust boundaries, public APIs, package formats, runtime isolation, or gateway behavior should include an Architecture Decision Record under `docs/adrs/`.

## Certificate of origin

By contributing, you certify that you have the right to submit the work under the project's Apache-2.0 license. A formal DCO check will be added before the first public beta.
