# Security Policy

## Supported versions

Security fixes are accepted on the default branch until the first stable release series is defined.

## Reporting a vulnerability

Please use GitHub private vulnerability reporting for this repository.

Do not open public issues that include exploit details, credentials, private endpoints, or production logs.

## Security model summary

- Registry metadata, packages, containers, skills, and remote endpoints are untrusted inputs.
- High-risk deployment plans require explicit approval before apply.
- Admin and gateway credentials are stored as one-way hashes.
- Secret values are never returned by list/read APIs.
- Managed containers default to non-root, dropped capabilities, no-new-privileges, and resource limits.
- Downstream tools are denied until included in an approved profile allowlist.
