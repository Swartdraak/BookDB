-- S5: Rich multilingual and multi-format catalog experience.
-- Forward-only migration. Adds audio performance fields, series membership,
-- aggregate contents, accessibility, and eligible assets.

-- Series membership: link a work to a series with an ordered position.
CREATE TABLE IF NOT EXISTS bookdb.series_members (
    member_id     UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    series_id     UUID NOT NULL REFERENCES bookdb.series(series_id) ON DELETE CASCADE,
    work_id       UUID NOT NULL REFERENCES bookdb.works(work_id) ON DELETE CASCADE,
    position      INTEGER,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (series_id, work_id)
);

CREATE INDEX IF NOT EXISTS idx_series_members_series ON bookdb.series_members (series_id, position);
CREATE INDEX IF NOT EXISTS idx_series_members_work ON bookdb.series_members (work_id);

-- Audio performance fields: narration metadata for audio expressions.
CREATE TABLE IF NOT EXISTS bookdb.audio_performances (
    performance_id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    expression_id  UUID NOT NULL REFERENCES bookdb.expressions(expression_id) ON DELETE CASCADE,
    narrator_id    UUID REFERENCES bookdb.people(person_id),
    producer_id    UUID REFERENCES bookdb.organizations(organization_id),
    is_abridged    BOOLEAN NOT NULL DEFAULT false,
    duration_sec   INTEGER,
    sample_url     TEXT,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (expression_id, narrator_id)
);

CREATE INDEX IF NOT EXISTS idx_audio_performances_expr ON bookdb.audio_performances (expression_id);

-- Accessibility: accessibility features for editions.
CREATE TABLE IF NOT EXISTS bookdb.edition_accessibility (
    accessibility_id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    edition_id       UUID NOT NULL REFERENCES bookdb.editions(edition_id) ON DELETE CASCADE,
    feature          TEXT NOT NULL
                     CHECK (feature IN ('braille', 'large_print', 'screen_reader', 'dyslexic_font', 'high_contrast', 'audio_description')),
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (edition_id, feature)
);

CREATE INDEX IF NOT EXISTS idx_edition_accessibility_edition ON bookdb.edition_accessibility (edition_id);

-- Assets: eligible covers, portraits, and audio samples.
CREATE TABLE IF NOT EXISTS bookdb.assets (
    asset_id       UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    entity_type    TEXT NOT NULL,
    entity_id      UUID NOT NULL,
    asset_kind     TEXT NOT NULL
                   CHECK (asset_kind IN ('cover', 'portrait', 'audio_sample', 'other')),
    url            TEXT NOT NULL,
    content_type   TEXT,
    byte_size      BIGINT,
    sha256         TEXT,
    eligibility    TEXT NOT NULL DEFAULT 'public'
                   CHECK (eligibility IN ('public', 'private', 'restricted')),
    source_name    TEXT,
    source_key     TEXT,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (entity_type, entity_id, asset_kind, url)
);

CREATE INDEX IF NOT EXISTS idx_assets_entity ON bookdb.assets (entity_type, entity_id);
CREATE INDEX IF NOT EXISTS idx_assets_eligibility ON bookdb.assets (eligibility);

-- Contributor roles: extended contributor metadata (imprint, studio, etc.).
-- The credits table already carries role; this adds a controlled vocabulary
-- reference for the richer S5 contributor model.
CREATE TABLE IF NOT EXISTS bookdb.contributor_roles (
    role_code      TEXT PRIMARY KEY,
    display_name   TEXT NOT NULL,
    description    TEXT,
    is_default     BOOLEAN NOT NULL DEFAULT false
);

-- Seed the controlled contributor role vocabulary.
INSERT INTO bookdb.contributor_roles (role_code, display_name, description, is_default) VALUES
    ('author', 'Author', 'Wrote the work', true),
    ('translator', 'Translator', 'Translated the expression', true),
    ('narrator', 'Narrator', 'Performed the audio narration', true),
    ('producer', 'Producer', 'Produced the audio or edition', true),
    ('publisher', 'Publisher', 'Published the edition', true),
    ('illustrator', 'Illustrator', 'Illustrated the edition', false),
    ('editor', 'Editor', 'Edited the work', false),
    ('introducer', 'Introducer', 'Wrote the introduction', false),
    ('foreword', 'Foreword', 'Wrote the foreword', false),
    ('afterword', 'Afterword', 'Wrote the afterword', false)
ON CONFLICT (role_code) DO NOTHING;

