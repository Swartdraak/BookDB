# Third-Party License Notices
#
# This file records the third-party software embedded in BookDB and its
# license. It is updated as a release gate (see
# `project-management/RELEASE_CHECKLIST.md`). Auto-generated SBOMs (CycloneDX)
# are produced for each release and archived with the release tag; this file
# is a human-curated summary so that contributors can reason about new
# dependencies at review time.
#
# Licensing policy reference: docs/24_LICENSING_POLICY.md
# Full license texts:
#   - AGPL-3.0-or-later (server) — `LICENSE`
#   - Apache-2.0 (SDKs / API clients) — `LICENSES/Apache-2.0.txt`
#   - CC BY-SA 4.0 (documentation) — `LICENSES/CC-BY-SA-4.0.txt`
#
# Format: one section per dependency; each entry lists module (Go module path
# or npm package name), version (as pinned), license SPDX identifier, and a
# short reason for inclusion.

## Go — primary server/runtime
<!-- populated during M1; entries are added as first-party go.mod pins -->

## Go — SDKs / generated clients
<!-- generated OpenAPI clients (Apache-2.0) and any external HTTP client
     dependencies go here. -->

## WebUI — npm / pnpm
<!-- populated during M1 -->

## Fonts
<!-- If any third-party fonts are shipped with the WebUI, their licenses
     must be recorded here. BookDB ships no custom fonts by default. -->

## Documentation
## Tooling

## Object storage reference (SeaweedFS)

SeaweedFS is the reference FOSS S3-compatible object storage for development
and reference HA deployments. It is NOT linked into BookDB — it is only run
out-of-process by `compose.yaml`. License: See
`deployment/compose/NOTES.md` for the current SeaweedFS release pin and its
license (GPL-3.0-or-later; operated as an independent service, separate from
the BookDB AGPL server, no AGPL cross-linkage is created by the S3 API).
