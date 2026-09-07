# BookDB SDK / API-Client License Notice

BookDB server, workers, scheduler, WebUI, and core CLI are licensed under the
**GNU Affero General Public License v3.0 or later (AGPL-3.0-or-later)**. See
`LICENSE` for the full license text.

Some BookDB artifacts are deliberately distributed under a **permissive
license** so that third parties — including proprietary consumers of the
public API — can integrate with BookDB without being bound by the server's
copyleft terms. This is a deliberate, narrow exception and is enforced at
package boundaries described below.

## Apache-2.0 artifacts

The following BookDB packages are licensed under **Apache License 2.0**
(see `LICENSES/Apache-2.0.txt`) and are *NOT* AGPL:

- `@bookdb/sdk` — the official TypeScript/JavaScript SDK for the BookDB API,
  published to npm.
- `@bookdb/cli` — the official CLI for common BookDB operations, published
  to npm. (Any server-derived code embedded in `@bookdb/cli` retains the
  AGPL license on that code; the CLI as a package is Apache-2.0.)
- Generated OpenAPI-derived clients for any language (e.g. a Go client for
  `api/openapi.yaml`) that are published as standalone artifacts.

### Rationale

- BookDB is a network service with a public REST API. Consumers of the API
  should be able to integrate without inheriting AGPL obligations on their
  own applications' source.
- The SDK and CLI are small, standalone packages that do not include
  server-side logic. Their Apache-2.0 license does not taint BookDB's
  AGPL core because the API contract is a stable public interface.

### Boundaries

- Do not publish BookDB server code (anything under `cmd/`, `internal/`,
  `migrations/`, `schemas/`) as a separately-packaged artifact. Those remain
  AGPL.
- If you add new SDK features, they live in a new `sdk/` subdirectory with a
  `LICENSE` file that reads "Apache License 2.0"; the server is unaffected.
- Any new generated OpenAPI client (TS, Go, Python, etc.) that is published
  to a public registries is Apache-2.0; any generated client that ships
  inside the BookDB repo but is NOT published is AGPL by default.

## Documentation

Documentation is licensed **CC BY-SA 4.0** unless marked otherwise in the
file or directory. BookDB-originated factual book-metadata curation may be
marked **CC0 1.0** where practical. See `docs/24_LICENSING_POLICY.md`.

## Third-party licenses

See `THIRD_PARTY_NOTICES.md` for the dependency-by-dependency license
summary, which is updated on every release.
