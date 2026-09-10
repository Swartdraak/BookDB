-- BookDB S1 catalog evolution.
--
-- Forward migration from the M1 foundation (0002). It preserves all existing
-- rows and relationships while introducing the typed, conflict-capable
-- identity model required by S1:
--
--   * organizations and contributor credits (typed, FK-enforced)
--   * namespaced identifiers with a conflict-capable lookup index
--   * edition contents (ordered junction) for multi-expression publications
--   * source records / revisions (evidence anchors)
--   * canonical field selection (traceable published values)
--   * API keys (keyed-hash storage, scopes, expiry, revocation)
--
-- It also relaxes two foundation constraints that the data model says are
-- incorrect:
--   * expressions: same-language distinct performances must not collide
--   * editions: a single unique isbn13 must not be the identity authority
--
-- Existing 0001/0002 files are never edited.

-- ---------------------------------------------------------------------------
-- 1. Organizations
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS bookdb.organizations (
    organization_id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    display_name text NOT NULL,
    sort_name text,
    organization_kind text,
    parent_id uuid REFERENCES bookdb.organizations(organization_id) ON DELETE SET NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_organizations_sort_name
    ON bookdb.organizations (sort_name);

-- ---------------------------------------------------------------------------
-- 2. Contributor credits (typed junction; no unchecked subject_type/subject_id)
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS bookdb.credits (
    credit_id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    person_id uuid REFERENCES bookdb.people(person_id) ON DELETE CASCADE,
    organization_id uuid REFERENCES bookdb.organizations(organization_id) ON DELETE CASCADE,
    role text NOT NULL,
    target_type text NOT NULL,
    target_id uuid NOT NULL,
    display_order integer NOT NULL DEFAULT 0,
    credited_as text,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT chk_credits_actor CHECK (person_id IS NOT NULL OR organization_id IS NOT NULL),
    CONSTRAINT chk_credits_target CHECK (target_type IN ('work', 'expression', 'edition')),
    CONSTRAINT uq_credits UNIQUE (person_id, organization_id, role, target_type, target_id, display_order)
);

CREATE INDEX IF NOT EXISTS idx_credits_target
    ON bookdb.credits (target_type, target_id);
CREATE INDEX IF NOT EXISTS idx_credits_person
    ON bookdb.credits (person_id);
CREATE INDEX IF NOT EXISTS idx_credits_organization
    ON bookdb.credits (organization_id);

-- ---------------------------------------------------------------------------
-- 3. Namespaced identifiers (conflict-capable)
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS bookdb.identifiers (
    identifier_id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    namespace text NOT NULL,
    normalized_value text NOT NULL,
    raw_value text,
    target_type text NOT NULL,
    target_id uuid NOT NULL,
    status text NOT NULL DEFAULT 'candidate',
    evidence jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT chk_identifiers_target CHECK (target_type IN ('work', 'expression', 'edition', 'person', 'organization')),
    CONSTRAINT chk_identifiers_status CHECK (status IN ('candidate', 'resolved', 'conflict', 'invalid')),
    CONSTRAINT uq_identifiers UNIQUE (namespace, normalized_value, target_type, target_id)
);

-- The lookup index is intentionally non-unique: conflicting claims across
-- sources must be preserved, and resolution may return ambiguity.
CREATE INDEX IF NOT EXISTS idx_identifiers_lookup
    ON bookdb.identifiers (namespace, normalized_value, target_type, target_id);
CREATE INDEX IF NOT EXISTS idx_identifiers_target
    ON bookdb.identifiers (target_type, target_id);

-- ---------------------------------------------------------------------------
-- 4. Edition contents (ordered junction between edition and expressions)
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS bookdb.edition_contents (
    content_id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    edition_id uuid NOT NULL REFERENCES bookdb.editions(edition_id) ON DELETE CASCADE,
    expression_id uuid NOT NULL REFERENCES bookdb.expressions(expression_id) ON DELETE CASCADE,
    position integer NOT NULL DEFAULT 0,
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (edition_id, expression_id, position)
);

CREATE INDEX IF NOT EXISTS idx_edition_contents_edition
    ON bookdb.edition_contents (edition_id);
CREATE INDEX IF NOT EXISTS idx_edition_contents_expression
    ON bookdb.edition_contents (expression_id);

-- ---------------------------------------------------------------------------
-- 5. Source records / revisions (evidence anchors)
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS bookdb.source_records (
    source_record_id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    source_name text NOT NULL,
    source_key text NOT NULL,
    content_hash text,
    acquired_at timestamptz NOT NULL DEFAULT now(),
    source_modified_at timestamptz,
    policy_ref text,
    payload jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (source_name, source_key)
);

CREATE INDEX IF NOT EXISTS idx_source_records_source
    ON bookdb.source_records (source_name, source_key);

-- ---------------------------------------------------------------------------
-- 6. Canonical field selection (traceable published values)
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS bookdb.canonical_field_selections (
    selection_id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    target_type text NOT NULL,
    target_id uuid NOT NULL,
    field text NOT NULL,
    selected_value jsonb NOT NULL,
    reason text,
    algorithm_version text,
    admin_override boolean NOT NULL DEFAULT false,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT chk_selections_target CHECK (target_type IN ('work', 'expression', 'edition', 'person', 'organization'))
);

CREATE INDEX IF NOT EXISTS idx_selections_target
    ON bookdb.canonical_field_selections (target_type, target_id, field);

-- ---------------------------------------------------------------------------
-- 7. API keys (keyed-hash storage; secret shown once, never stored in clear)
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS bookdb.api_keys (
    key_id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    lookup_prefix text NOT NULL UNIQUE,
    key_hash text NOT NULL,
    name text NOT NULL,
    scopes text[] NOT NULL DEFAULT '{}',
    owner text,
    created_at timestamptz NOT NULL DEFAULT now(),
    expires_at timestamptz,
    revoked_at timestamptz,
    last_used_at timestamptz
);

CREATE INDEX IF NOT EXISTS idx_api_keys_owner
    ON bookdb.api_keys (owner);

-- ---------------------------------------------------------------------------
-- 8. Relax foundation constraints that the data model says are incorrect.
-- ---------------------------------------------------------------------------
-- Same-language distinct performances (e.g. two narrations) must not collide.
ALTER TABLE bookdb.expressions DROP CONSTRAINT IF EXISTS expressions_work_id_language_code_expression_title_key;

-- A single unique isbn13 must not be the identity authority; identifiers are
-- conflict-capable. Existing populated values are preserved (no data loss).
ALTER TABLE bookdb.editions DROP CONSTRAINT IF EXISTS uq_editions_isbn13;

