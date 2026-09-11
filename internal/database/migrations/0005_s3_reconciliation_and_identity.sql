-- S3: Cross-source reconciliation, identity operations, and change feed.
-- Forward-only migration. Does not modify existing tables.

-- Canonical revisions: track the version of each canonical entity for
-- optimistic concurrency control in merge/split operations.
CREATE TABLE IF NOT EXISTS bookdb.canonical_revisions (
    entity_type   TEXT NOT NULL,
    entity_id     UUID NOT NULL,
    revision      BIGINT NOT NULL DEFAULT 1,
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (entity_type, entity_id)
);

-- Identity redirects: when two entities are merged, the old ID redirects
-- to the new canonical ID. Splits create reverse redirects.
CREATE TABLE IF NOT EXISTS bookdb.identity_redirects (
    redirect_id   UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    from_type     TEXT NOT NULL,
    from_id       UUID NOT NULL,
    to_type       TEXT NOT NULL,
    to_id         UUID NOT NULL,
    reason        TEXT NOT NULL DEFAULT 'merge',
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (from_type, from_id)
);

CREATE INDEX IF NOT EXISTS idx_identity_redirects_to
    ON bookdb.identity_redirects (to_type, to_id);

-- Duplicate candidates: pairs of entities that may be the same, pending
-- review or automatic reconciliation.
CREATE TABLE IF NOT EXISTS bookdb.duplicate_candidates (
    candidate_id  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    type_a        TEXT NOT NULL,
    id_a          UUID NOT NULL,
    type_b        TEXT NOT NULL,
    id_b          UUID NOT NULL,
    confidence    NUMERIC(5,4) NOT NULL DEFAULT 0,
    evidence      JSONB NOT NULL DEFAULT '[]'::jsonb,
    status        TEXT NOT NULL DEFAULT 'pending'
                  CHECK (status IN ('pending', 'confirmed', 'rejected', 'merged', 'split')),
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    resolved_at   TIMESTAMPTZ,
    UNIQUE (type_a, id_a, type_b, id_b)
);

CREATE INDEX IF NOT EXISTS idx_duplicate_candidates_status
    ON bookdb.duplicate_candidates (status);

-- Field-level provenance: track which source provided each canonical field
-- value, enabling field-level selection and contradiction detection.
CREATE TABLE IF NOT EXISTS bookdb.field_provenance (
    provenance_id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    entity_type   TEXT NOT NULL,
    entity_id     UUID NOT NULL,
    field_name    TEXT NOT NULL,
    source_name   TEXT NOT NULL,
    source_key    TEXT NOT NULL,
    value         JSONB NOT NULL,
    selected      BOOLEAN NOT NULL DEFAULT false,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (entity_type, entity_id, field_name, source_name, source_key)
);

CREATE INDEX IF NOT EXISTS idx_field_provenance_entity
    ON bookdb.field_provenance (entity_type, entity_id, field_name);

-- Change feed: durable, ordered log of all canonical state changes.
-- Clients consume from a cursor (last consumed ID) and receive changes
-- in committed order. The feed is append-only.
CREATE TABLE IF NOT EXISTS bookdb.change_feed (
    change_id     BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    entity_type   TEXT NOT NULL,
    entity_id     UUID NOT NULL,
    change_type   TEXT NOT NULL
                  CHECK (change_type IN ('created', 'updated', 'merged', 'split', 'deleted')),
    payload       JSONB NOT NULL DEFAULT '{}'::jsonb,
    revision      BIGINT NOT NULL DEFAULT 1,
    committed_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_change_feed_entity
    ON bookdb.change_feed (entity_type, entity_id, committed_at DESC);
CREATE INDEX IF NOT EXISTS idx_change_feed_cursor
    ON bookdb.change_feed (change_id);

-- Wikidata source records: extend source_records with Wikidata-specific
-- fields via a separate table for the second source.
CREATE TABLE IF NOT EXISTS bookdb.wikidata_claims (
    claim_id      UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    entity_type   TEXT NOT NULL,
    entity_id     UUID NOT NULL,
    wikidata_qid  TEXT NOT NULL,
    property      TEXT NOT NULL,
    value         JSONB NOT NULL,
    source_rank   TEXT NOT NULL DEFAULT 'normal',
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (entity_type, entity_id, wikidata_qid, property)
);

CREATE INDEX IF NOT EXISTS idx_wikidata_claims_qid
    ON bookdb.wikidata_claims (wikidata_qid);

