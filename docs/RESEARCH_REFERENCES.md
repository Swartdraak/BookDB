# Research References and Current Technical Baseline

Checked: 2026-09-06.

## Licensing

- Open Source Initiative — AGPL-3.0 is OSI-approved:
  https://opensource.org/license/agpl-3-0
- GNU license recommendations — recommends AGPL for server software where hosted modifications should remain available:
  https://www.gnu.org/licenses/license-recommendations.html
- GNU AGPL guidance:
  https://www.gnu.org/licenses/why-affero-gpl.html.en

## Go

- Go release history:
  https://go.dev/doc/devel/release
- Current researched baseline: Go 1.26.8, released 2026-09-01.

## PostgreSQL

- PostgreSQL supported versions:
  https://www.postgresql.org/support/versioning/
- PostgreSQL 18 HA/replication:
  https://www.postgresql.org/docs/18/high-availability.html
- Current researched baseline: PostgreSQL 18.6.

## Patroni/PgBouncer

- Patroni:
  https://patroni.readthedocs.io/
  https://github.com/patroni/patroni
- PgBouncer:
  https://www.pgbouncer.org/

## NATS / JetStream

- NATS docs:
  https://docs.nats.io/
- JetStream concepts/clustering:
  https://docs.nats.io/nats-concepts/jetstream
  https://docs.nats.io/running-a-nats-service/configuration/clustering/jetstream_clustering
- NATS server license/repository:
  https://github.com/nats-io/nats-server
- Research baseline: NATS is Apache-2.0; JetStream clustering uses RAFT and recommends 3 or 5 JetStream-enabled servers for HA.

## Valkey

- https://valkey.io/
- https://valkey.io/download/
- https://valkey.io/topics/cluster-spec/
- Current researched baseline: Valkey 9.1.2 released 2026-09-01.

## OpenSearch

- https://opensearch.org/
- https://docs.opensearch.org/latest/
- https://github.com/opensearch-project/OpenSearch
- Apache-2.0 distributed search engine.
- OpenSearch documentation states three cluster-manager-eligible nodes can tolerate one such node failure.

## Object storage

- SeaweedFS:
  https://github.com/seaweedfs/seaweedfs
  Apache-2.0, distributed S3-compatible storage intended for very high object counts.
- MinIO:
  https://github.com/minio/minio
  The upstream open-source repository was archived in April 2026, so BookDB no longer uses it as the reference object-store recommendation.
- BookDB uses S3 semantics so operators may choose another compatible implementation.

## Frontend

- React 19.2:
  https://react.dev/blog/2025/10/01/react-19-2
- React security/release blog:
  https://react.dev/blog
- Vite 8.1:
  https://vite.dev/blog/announcing-vite8-1
- TanStack Query v5:
  https://tanstack.com/query/latest/docs/framework/react

## Bibliographic and metadata research

- Open Library:
  https://openlibrary.org/developers/api
  https://openlibrary.org/developers/dumps
  https://openlibrary.org/help/faq/using
- BIBFRAME:
  https://www.loc.gov/bibframe/docs/bibframe2-model.html
- MARC 21:
  https://www.loc.gov/marc/bibliographic/
- LOC SRU/Z39.50:
  https://www.loc.gov/standards/z3950/lcserver.html
- Wikidata licensing:
  https://www.wikidata.org/wiki/Wikidata:Licensing
- Schema.org:
  https://schema.org/Book
  https://schema.org/Audiobook
- Crossref:
  https://www.crossref.org/documentation/retrieve-metadata/rest-api/
- DataCite:
  https://support.datacite.org/docs/rest-api
- Europeana:
  https://www.europeana.eu/en/rights/terms-of-use
- DOAB:
  https://www.doabooks.org/en/resources/metadata-harvesting-and-content-dissemination
- VIAF:
  https://www.oclc.org/developer/api/oclc-apis/viaf.en.html
- LibriVox:
  https://librivox.org/api/info
- Goodreads terms:
  https://www.goodreads.com/about/terms
- Google API terms:
  https://developers.google.com/terms

Third-party terms/licensing must be revalidated before connector release.
