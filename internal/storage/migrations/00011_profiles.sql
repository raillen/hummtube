-- M9 perfis locais isolados (QOL-04): cada perfil tem seu histórico/playlists/favoritos.
-- +goose Up
CREATE TABLE profiles (
    id         TEXT PRIMARY KEY,
    name       TEXT NOT NULL,
    created_at TEXT NOT NULL DEFAULT ''
);
CREATE TABLE profile_members (
    profile_id TEXT NOT NULL REFERENCES profiles(id) ON DELETE CASCADE,
    kind       TEXT NOT NULL,
    ref_id     TEXT NOT NULL,
    PRIMARY KEY (profile_id, kind, ref_id)
);

-- +goose Down
DROP TABLE IF EXISTS profile_members;
DROP TABLE IF EXISTS profiles;
