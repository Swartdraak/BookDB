-- BookDB M1 canonical schema foundation.
--
-- This migration establishes a minimal, forward-compatible canonical model for
-- bibliographic identity and governance evidence. It is intentionally skeletal
-- (no advanced reconciliation/materialized projections yet) but provides stable
-- relational anchors for later milestones.

CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE IF NOT EXISTS bookdb.works (
    work_id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    canonical_title text NOT NULL,
    normalized_title text NOT NULL,
    language_code text,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_works_normalized_title
    ON bookdb.works (normalized_title);

CREATE TABLE IF NOT EXISTS bookdb.expressions (
    expression_id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    work_id uuid NOT NULL REFERENCES bookdb.works(work_id) ON DELETE CASCADE,
    language_code text NOT NULL,
    expression_title text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (work_id, language_code, expression_title)
);

CREATE INDEX IF NOT EXISTS idx_expressions_work
    ON bookdb.expressions (work_id);

CREATE TABLE IF NOT EXISTS bookdb.editions (
    edition_id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    expression_id uuid NOT NULL REFERENCES bookdb.expressions(expression_id) ON DELETE CASCADE,
    edition_title text NOT NULL,
    format text,
    isbn13 text,
    publication_date date,
    publisher_name text,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT uq_editions_isbn13 UNIQUE (isbn13)
);

CREATE INDEX IF NOT EXISTS idx_editions_expression
    ON bookdb.editions (expression_id);

CREATE TABLE IF NOT EXISTS bookdb.market_listings (
    listing_id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    edition_id uuid NOT NULL REFERENCES bookdb.editions(edition_id) ON DELETE CASCADE,
    source_name text NOT NULL,
    source_record_id text NOT NULL,
    availability_status text NOT NULL DEFAULT 'unknown',
    price_amount numeric(12, 2),
    currency_code char(3),
    listed_at timestamptz NOT NULL DEFAULT now(),
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (source_name, source_record_id)
);

CREATE INDEX IF NOT EXISTS idx_market_listings_edition
    ON bookdb.market_listings (edition_id);

CREATE TABLE IF NOT EXISTS bookdb.people (
    person_id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    display_name text NOT NULL,
    sort_name text,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS bookdb.series (
    series_id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    canonical_title text NOT NULL,
    normalized_title text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS bookdb.sources (
    source_id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    source_name text NOT NULL UNIQUE,
    enabled boolean NOT NULL DEFAULT true,
    policy_ref text,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS bookdb.claims (
    claim_id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    source_name text NOT NULL,
    source_record_id text NOT NULL,
    subject_type text NOT NULL,
    subject_id uuid NOT NULL,
    predicate text NOT NULL,
    object_value jsonb NOT NULL,
    confidence numeric(5, 4) NOT NULL DEFAULT 0.5000,
    observed_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT chk_claims_confidence CHECK (confidence >= 0.0 AND confidence <= 1.0)
);

CREATE INDEX IF NOT EXISTS idx_claims_subject
    ON bookdb.claims (subject_type, subject_id);

CREATE INDEX IF NOT EXISTS idx_claims_source
    ON bookdb.claims (source_name, source_record_id);

CREATE TABLE IF NOT EXISTS bookdb.proposals (
    proposal_id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    proposal_type text NOT NULL,
    proposed_by text NOT NULL,
    status text NOT NULL DEFAULT 'pending',
    payload jsonb NOT NULL,
    rationale text,
    created_at timestamptz NOT NULL DEFAULT now(),
    decided_at timestamptz,
    decided_by text,
    CONSTRAINT chk_proposals_status CHECK (status IN ('pending', 'approved', 'rejected', 'superseded'))
);

CREATE TABLE IF NOT EXISTS bookdb.audit_log (
    audit_id bigserial PRIMARY KEY,
    actor text NOT NULL,
    action text NOT NULL,
    entity_type text,
    entity_id text,
    details jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_audit_log_entity
    ON bookdb.audit_log (entity_type, entity_id);

