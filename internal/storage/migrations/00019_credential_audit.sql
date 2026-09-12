-- Metadados de auditoria de credenciais. Valores secretos nunca entram aqui.
-- +goose Up
CREATE TABLE credential_audit (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    profile_id  TEXT NOT NULL REFERENCES profiles(id) ON DELETE CASCADE,
    provider    TEXT NOT NULL,
    action      TEXT NOT NULL CHECK (action IN ('connected', 'rotated', 'revoked', 'verification_failed')),
    occurred_at TEXT NOT NULL,
    detail      TEXT NOT NULL DEFAULT ''
);

CREATE INDEX credential_audit_profile_time_idx
    ON credential_audit(profile_id, occurred_at DESC, id DESC);

-- +goose Down
DROP INDEX IF EXISTS credential_audit_profile_time_idx;
DROP TABLE IF EXISTS credential_audit;
