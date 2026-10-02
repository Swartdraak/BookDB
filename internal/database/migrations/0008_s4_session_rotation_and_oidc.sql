-- S4 (issue #28): session role rotation + OIDC account-linking identity split.
--
-- 1. sessions.issued_role: the role the session was ISSUED with. A
--    privilege change made outside the session (BDB-007/009) must not
--    silently extend the session's privileges; ValidateSession rejects a
--    session whose issued role no longer matches the account so the user
--    rotates with a fresh login.
--
-- 2. users.oidc_issuer: the OIDC identity is the PAIR (issuer, subject),
--    not the subject alone — the same subject at two issuers is a distinct
--    identity (BDB-010). Account linking keys on both, with a uniqueness
--    constraint on the pair (partial index; NULL subject rows unaffected).
--
-- Forward-only migration.

ALTER TABLE bookdb.sessions
    ADD COLUMN IF NOT EXISTS issued_role TEXT;

ALTER TABLE bookdb.users
    ADD COLUMN IF NOT EXISTS oidc_issuer TEXT;

CREATE UNIQUE INDEX IF NOT EXISTS idx_users_oidc_issuer_subject
    ON bookdb.users (oidc_issuer, oidc_subject)
    WHERE oidc_subject IS NOT NULL;
