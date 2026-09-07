# BookDB Licensing Policy

## BookDB application

**SPDX:** `AGPL-3.0-or-later`

Applies to:
- server;
- workers;
- scheduler;
- WebUI when distributed as part of BookDB;
- core CLI;
- database migration/application code.

## Why AGPL

BookDB is designed to run as a network service. Strong network copyleft matches the project goal that organizations should not take BookDB, modify it, operate the modified service for users, and permanently withhold those server modifications from the community.

## Client integration boundary

Official client SDKs should use **Apache-2.0**:
- TypeScript SDK;
- Go API client if separately published;
- generated OpenAPI clients;
- generic compatibility adapter SDK.

This makes the BookDB HTTP API straightforward for a wide range of consumers. Merely communicating with an AGPL server over its API does not turn a separate client into BookDB server code; exact license questions for derivative works should still be reviewed by adopters.

## Documentation

Recommended: **CC BY-SA 4.0**.

## Database and metadata

Software licensing does not erase source data rights.

BookDB tracks:
- source metadata license;
- attribution;
- database rights;
- image/content rights;
- redistributability.

No release should claim “all BookDB data is CC0” unless the export is generated from a policy-filtered set that can legally carry that designation.

## Dependency policy

Preferred dependency licenses:
- Apache-2.0
- MIT
- BSD
- ISC
- PostgreSQL License
- MPL-2.0 where isolated/compatible

Copyleft dependencies are reviewed for compatibility.

## Repository files

The repository should include:
- `LICENSE` — AGPL text;
- `LICENSES/Apache-2.0.txt` for SDKs;
- `LICENSES/CC-BY-SA-4.0.txt` or link/process for docs;
- SPDX headers or package metadata where practical;
- `THIRD_PARTY_NOTICES.md`;
- SBOM for releases.
