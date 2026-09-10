# BookDB

A self-hosted bibliographic metadata authority for books, ebooks and audiobooks, with open-source ingestion, a modern WebUI and a documented API.

Start with **[BOOKDB_PROJECT.md](BOOKDB_PROJECT.md)** for requirements, architecture, delivery stages, testing, GitHub operations and agent setup.

## Current implementation status

The inspected baseline contains Go database/runtime/authentication scaffolding, service adapters and a React shell. It does not yet provide the complete catalog product. The package adoption stage must restore the missing runtime entry point and verify installation before the application is described as runnable. GitHub Milestones and Issues hold current progress; this paragraph should be updated in the S0 delivery PR with verified startup commands.

## Develop

Read [CONTRIBUTING.md](CONTRIBUTING.md). AI agents start with [AGENTS.md](AGENTS.md). The authoritative command/status table is in [testing](docs/bookdb/08-testing.md). Do not confuse a documented target API with an implemented endpoint.

## License

Retain the repository's [LICENSE](LICENSE), [SDK_LICENSE.md](SDK_LICENSE.md) and [LICENSES/README.md](LICENSES/README.md). Server/WebUI/workers/core CLI: AGPL-3.0-or-later. Official SDK/API definitions: Apache-2.0. Source metadata and assets have separate rights.
