# Source Policy and Metadata Research

## Purpose

BookDB is intended to aggregate a global bibliographic catalog, but technical accessibility does not grant permission to permanently copy or redistribute a source.

Every connector is controlled by a versioned source-policy record.

## Policy states

- `approved_bulk`
- `approved_api_persistent`
- `approved_attribution_required`
- `query_only_no_persistence`
- `legal_review_required`
- `blocked`

## Required source-policy fields

- source ID/name
- first-party terms URL
- API/bulk documentation URL
- metadata license
- database-rights notes
- allowed acquisition modes
- allowed persistence scope
- raw-record retention
- normalized-claim retention
- redistribution/export obligations
- attribution requirements
- asset/image policy
- request/rate guidance
- allowed hostnames
- reviewed date
- next review date
- maintainer/approver
- policy status

## Enforcement

A connector cannot start until its policy is loaded.

Policy enforcement occurs:
1. before scheduling;
2. before network acquisition;
3. before raw persistence;
4. before canonical claim eligibility;
5. before asset persistence;
6. before export.

A source marked `blocked` cannot be enabled from the UI alone.

A source marked `query_only_no_persistence` may be used only for explicitly approved transient validation workflows and must not create durable source claims.

## Initial research classification

### Open Library
Status: `approved_bulk`.

Strengths:
- monthly dumps intended for bulk use;
- broad authors/works/editions coverage;
- useful foundational identity/evidence.

Weakness:
- duplicates and uneven field completeness.

Role:
bootstrap evidence, not canonical authority.

### Wikidata
Status: `approved_bulk`.

Structured data is CC0.

Role:
- authority linkage;
- multilingual names;
- external identifiers;
- relationships.

### Europeana
Status: `approved_api_persistent` for eligible metadata.

Metadata and object rights are separate. Digital assets must preserve the rights statement applicable to each item.

### DOAB
Status: `approved_bulk`.

Useful for open-access scholarly books and structured publisher metadata.

### Crossref
Status: `approved_api_persistent` with field-level rights review.

Good for DOI-bearing books/chapters. Abstracts/content can carry different rights from basic bibliographic facts.

### DataCite
Status: `legal_review_required` until BookDB's exact export/retention policy is validated against current terms.

### OpenAlex
Status: `approved_bulk` under current CC0 data model.

Best used for scholarly identity/enrichment, not as a universal trade-book authority.

### VIAF
Status: `approved_attribution_required`.

Useful for person/organization authority resolution. ODC-BY obligations must accompany relevant exports.

### Library of Congress
Status: `legal_review_required` per interface/dataset.

Technically valuable:
- MARC;
- MODS;
- BIBFRAME;
- SRU/Z39.50;
- authority data.

Digital-object rights must not be inferred from catalog-record access.

### LibriVox
Status: `legal_review_required` pending exact metadata/cover redistribution review.

Technically valuable for:
- audiobook narrator;
- sections;
- duration;
- public-domain audio catalog.

### Project Gutenberg
Status: `legal_review_required`.

Machine-readable catalog is intentionally published for tools/databases, but BookDB must validate current trademark/metadata terms before redistributing a mirror.

### Google Books
Status: `query_only_no_persistence` by default.

Current Google API terms restrict building permanent copies/databases of returned API content unless separately permitted. BookDB must not treat its API as a persistent canonical feed without separate permission.

### Goodreads
Status: `blocked` by default.

Current Goodreads terms prohibit automated collection/use of book listings/descriptions and data-mining/robot-style extraction absent permission.

## Source synchronization

Approved sources are not continuously polled.

Execution occurs only through:
- configured schedules;
- Administrator manual trigger;
- approved upstream event/webhook;
- bootstrap;
- recovery replay.

See `docs/21_SYNCHRONIZATION_SCHEDULING.md`.

## Field authority

A source can be strong for one field and weak for another.

Examples:
- author identity: authority files
- ISBN/publisher/date: publication/catalog evidence
- narrator/runtime: exact audiobook release
- series: publisher/author/structured corroboration
- cover: exact edition and rights-compatible source

BookDB never assigns one source a universal “wins everything” priority.

## Research references

See `docs/RESEARCH_REFERENCES.md`.
