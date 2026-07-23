# Security model

MCP servers, skills, prompts, catalog metadata, repositories, containers, and remote endpoints are untrusted inputs.

## Initial invariants

- No arbitrary repository is executed merely because it appears in a registry.
- Secrets are represented by opaque references and never returned by read APIs.
- High-risk plans require an explicit approval boundary.
- Downstream tools are denied to clients until included in an approved profile.
- Containerized servers run non-root with dropped capabilities and bounded resources by default.
- Artifact versions and digests are recorded before deployment.
- Logs and audit events must redact configured secret values.

## Threats considered

- Malicious or compromised MCP packages
- Prompt injection in skills and metadata
- Dependency and image supply-chain compromise
- Credential theft through logs, environment inspection, or tool output
- Over-broad filesystem and network access
- Tool-name collisions and confused-deputy routing
- Unauthorized profile or gateway changes
- Registry metadata drift and publisher impersonation

## Reporting vulnerabilities

Do not open public issues containing exploit details or credentials. Until a dedicated security address is established, use GitHub's private vulnerability reporting feature for the repository.
