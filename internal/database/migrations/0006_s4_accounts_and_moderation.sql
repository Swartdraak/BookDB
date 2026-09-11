-- S4: Accounts, sessions, RBAC, and moderation workflow.
-- Forward-only migration.

-- User accounts: local authentication with password hashing.
CREATE TABLE IF NOT EXISTS bookdb.users (
    user_id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    username        TEXT NOT NULL UNIQUE,
    email           TEXT NOT NULL UNIQUE,
    password_hash   TEXT NOT NULL,
    display_name    TEXT NOT NULL DEFAULT '',
    role            TEXT NOT NULL DEFAULT 'reader'
                    CHECK (role IN ('reader', 'contributor', 'administrator')),
    status          TEXT NOT NULL DEFAULT 'active'
                    CHECK (status IN ('active', 'disabled', 'pending')),
    oidc_subject    TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_login_at   TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_users_username ON bookdb.users (username);
CREATE INDEX IF NOT EXISTS idx_users_oidc ON bookdb.users (oidc_subject) WHERE oidc_subject IS NOT NULL;

-- Sessions: secure browser sessions with CSRF protection.
CREATE TABLE IF NOT EXISTS bookdb.sessions (
    session_id      UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id         UUID NOT NULL REFERENCES bookdb.users(user_id) ON DELETE CASCADE,
    csrf_token      TEXT NOT NULL,
    ip_address      TEXT,
    user_agent      TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at      TIMESTAMPTZ NOT NULL,
    last_used_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    revoked_at      TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_sessions_user ON bookdb.sessions (user_id);
CREATE INDEX IF NOT EXISTS idx_sessions_expires ON bookdb.sessions (expires_at);

-- Proposals: user-submitted corrections pending admin review.
-- Note: migration 0002 already creates bookdb.proposals with a different
-- schema. We use a new table name to avoid the conflict.
CREATE TABLE IF NOT EXISTS bookdb.correction_proposals (
    proposal_id     UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id         UUID NOT NULL REFERENCES bookdb.users(user_id),
    entity_type     TEXT NOT NULL,
    entity_id       UUID NOT NULL,
    field_name      TEXT NOT NULL,
    proposed_value  JSONB NOT NULL,
    current_value   JSONB,
    rationale       TEXT NOT NULL DEFAULT '',
    status          TEXT NOT NULL DEFAULT 'pending'
                    CHECK (status IN ('pending', 'approved', 'rejected', 'withdrawn')),
    revision        BIGINT NOT NULL DEFAULT 1,
    reviewed_by     UUID REFERENCES bookdb.users(user_id),
    reviewed_at     TIMESTAMPTZ,
    review_note     TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_correction_proposals_status ON bookdb.correction_proposals (status);
CREATE INDEX IF NOT EXISTS idx_correction_proposals_entity ON bookdb.correction_proposals (entity_type, entity_id);
CREATE INDEX IF NOT EXISTS idx_correction_proposals_user ON bookdb.correction_proposals (user_id);

-- Audit log: record all administrative actions.
CREATE TABLE IF NOT EXISTS bookdb.audit_events (
    event_id        BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    user_id         UUID,
    action          TEXT NOT NULL,
    entity_type     TEXT,
    entity_id       UUID,
    detail          JSONB NOT NULL DEFAULT '{}'::jsonb,
    ip_address      TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_audit_events_user ON bookdb.audit_events (user_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_audit_events_entity ON bookdb.audit_events (entity_type, created_at DESC);
