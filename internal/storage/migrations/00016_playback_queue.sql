-- Fila de reprodução persistente e isolada por perfil.
-- +goose Up
CREATE TABLE playback_queue (
    id         TEXT NOT NULL,
    profile_id TEXT NOT NULL REFERENCES profiles(id) ON DELETE CASCADE,
    video_id   TEXT NOT NULL REFERENCES videos(id) ON DELETE CASCADE,
    position   INTEGER NOT NULL CHECK (position >= 0),
    state      TEXT NOT NULL DEFAULT 'pending' CHECK (state IN ('pending', 'played')),
    added_at   TEXT NOT NULL,
    played_at  TEXT NOT NULL DEFAULT '',
    PRIMARY KEY (profile_id, id),
    UNIQUE (profile_id, video_id)
);
CREATE INDEX playback_queue_profile_position_idx
    ON playback_queue(profile_id, position, id);

CREATE TABLE playback_queue_preferences (
    profile_id    TEXT PRIMARY KEY REFERENCES profiles(id) ON DELETE CASCADE,
    autoplay      INTEGER NOT NULL DEFAULT 1 CHECK (autoplay IN (0, 1)),
    remove_played INTEGER NOT NULL DEFAULT 0 CHECK (remove_played IN (0, 1))
);

-- +goose Down
DROP TABLE IF EXISTS playback_queue_preferences;
DROP INDEX IF EXISTS playback_queue_profile_position_idx;
DROP TABLE IF EXISTS playback_queue;
