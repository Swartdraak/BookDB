-- S2: Ingestion job state, transactional outbox, and search projection support.
-- Forward-only migration. Does not modify existing tables.
-- Note: the migrator wraps each migration in its own transaction, so this file
-- must not contain explicit BEGIN/COMMIT statements.

-- Ingestion job state: tracks the progress of a source import run.
CREATE TABLE IF NOT EXISTS bookdb.ingestion_jobs (
    job_id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    source_name     TEXT NOT NULL,
    snapshot_id     TEXT NOT NULL,
    status          TEXT NOT NULL DEFAULT 'pending'
                    CHECK (status IN ('pending', 'running', 'paused', 'completed', 'failed', 'quarantined')),
    total_records   BIGINT NOT NULL DEFAULT 0,
    processed       BIGINT NOT NULL DEFAULT 0,
    accepted        BIGINT NOT NULL DEFAULT 0,
    unchanged       BIGINT NOT NULL DEFAULT 0,
    rejected        BIGINT NOT NULL DEFAULT 0,
    quarantined     BIGINT NOT NULL DEFAULT 0,
    checkpoint      JSONB,
    error_message   TEXT,
    started_at      TIMESTAMPTZ,
    finished_at     TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_ingestion_jobs_status
    ON bookdb.ingestion_jobs (status);
CREATE INDEX IF NOT EXISTS idx_ingestion_jobs_source
    ON bookdb.ingestion_jobs (source_name, created_at DESC);

-- Transactional outbox: durable events that need to be published to NATS
-- and/or applied to the search projection. Written in the same transaction
-- as the canonical state change.
CREATE TABLE IF NOT EXISTS bookdb.outbox (
    id              BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    aggregate_type  TEXT NOT NULL,
    aggregate_id    UUID NOT NULL,
    event_type      TEXT NOT NULL,
    payload         JSONB NOT NULL,
    published_at    TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_outbox_unpublished
    ON bookdb.outbox (id) WHERE published_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_outbox_aggregate
    ON bookdb.outbox (aggregate_type, aggregate_id, created_at DESC);

-- Source manifests: track the identity and hash of each imported snapshot.
CREATE TABLE IF NOT EXISTS bookdb.source_manifests (
    manifest_id     UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    source_name     TEXT NOT NULL,
    snapshot_id     TEXT NOT NULL,
    snapshot_hash   TEXT NOT NULL,
    record_count    BIGINT NOT NULL DEFAULT 0,
    retrieved_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    metadata        JSONB,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (source_name, snapshot_id)
);

-- Search projection state: tracks which canonical entities have been
-- indexed and their current revision for incremental updates.
CREATE TABLE IF NOT EXISTS bookdb.search_projection_state (
    entity_type     TEXT NOT NULL,
    entity_id       UUID NOT NULL,
    revision        BIGINT NOT NULL DEFAULT 1,
    indexed_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (entity_type, entity_id)
);
