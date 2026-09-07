# ADR 0010: Dependency Tooling for BookDB

## Status

Accepted

## Context

BookDB needs a reproducible, low-friction dependency and supply-chain baseline for a new repository foundation. The repository currently has Go tooling plus a devcontainer image, and the operational goal is to keep dependency updates and CI checks native to GitHub without adding a separate service to maintain.

## Decision

Use Dependabot for dependency updates and GitHub Actions for dependency-review, secret scanning, SBOM generation, and image vulnerability scans.

Dependabot will manage:

- Go modules;
- GitHub Actions references;
- the Dev Container Dockerfile under `.devcontainer/`.

Renovate is deferred for later unless the repository grows enough to justify a separate dependency automation service.

## Consequences

- Dependency updates are visible in standard GitHub PR flows.
- Security gates stay close to the repository and do not require external automation infrastructure.
- The repo can keep a single authoritative dependency policy in GitHub-native configuration.