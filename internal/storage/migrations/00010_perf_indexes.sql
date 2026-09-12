-- M8 PERF-01: índices compostos para bibliotecas e FTS já cobre busca
-- +goose Up
CREATE INDEX IF NOT EXISTS idx_videos_published ON videos(published_at DESC, id);
CREATE INDEX IF NOT EXISTS idx_videos_channel ON videos(channel_id, published_at DESC);
CREATE INDEX IF NOT EXISTS idx_playback_progress_updated ON playback_progress(updated_at DESC, video_id);
CREATE INDEX IF NOT EXISTS idx_queue_order ON queue_items(order_index, id);
CREATE INDEX IF NOT EXISTS idx_favorites_created ON favorites(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_recommendation_feedback_action ON recommendation_feedback(action);

-- +goose Down
DROP INDEX IF EXISTS idx_recommendation_feedback_action;
DROP INDEX IF EXISTS idx_favorites_created;
DROP INDEX IF EXISTS idx_queue_order;
DROP INDEX IF EXISTS idx_playback_progress_updated;
DROP INDEX IF EXISTS idx_videos_channel;
DROP INDEX IF EXISTS idx_videos_published;
