package storage

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/nanotube/nanotube-web/internal/domain"
)

const maxQueueItems = 100

func queueItemID() (string, error) {
	buffer := make([]byte, 6)
	if _, err := rand.Read(buffer); err != nil {
		return "", fmt.Errorf("fila: gerar id: %w", err)
	}
	return "queue_" + hex.EncodeToString(buffer), nil
}

// PlaybackQueue carrega a fila do perfil ativo na ordem definida pelo usuário.
func (r *Repository) PlaybackQueue(ctx context.Context) (domain.QueueSnapshot, error) {
	profileID, err := r.ActiveProfileID(ctx)
	if err != nil {
		return domain.QueueSnapshot{}, err
	}
	preferences, err := r.QueuePreferences(ctx)
	if err != nil {
		return domain.QueueSnapshot{}, err
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT q.id, q.position, q.state, q.added_at, q.played_at, `+videoColumns+`
		FROM playback_queue q
		JOIN videos v ON v.id = q.video_id
		WHERE q.profile_id = ?
		ORDER BY q.position, q.id`, profileID)
	if err != nil {
		return domain.QueueSnapshot{}, fmt.Errorf("fila: listar: %w", err)
	}
	defer rows.Close()

	items := make([]domain.QueueItem, 0)
	for rows.Next() {
		var item domain.QueueItem
		var state, addedAt, playedAt string
		var publishedAt, firstSeenAt, lastSeenAt string
		var durationSeconds int64
		if err := rows.Scan(
			&item.ID, &item.Position, &state, &addedAt, &playedAt,
			&item.Video.ID, &item.Video.ChannelID, &item.Video.Title,
			&item.Video.Description, &item.Video.DescriptionExcerpt, &publishedAt, &durationSeconds,
			&item.Video.Category, &item.Video.ThumbnailURL, &item.Video.LiveStatus,
			&firstSeenAt, &lastSeenAt,
		); err != nil {
			return domain.QueueSnapshot{}, fmt.Errorf("fila: ler item: %w", err)
		}
		item.State = domain.QueueItemState(state)
		item.AddedAt = parseTime(addedAt)
		item.PlayedAt = parseTime(playedAt)
		item.Video.PublishedAt = parseTime(publishedAt)
		item.Video.Duration = time.Duration(durationSeconds) * time.Second
		item.Video.FirstSeenAt = parseTime(firstSeenAt)
		item.Video.LastSeenAt = parseTime(lastSeenAt)
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return domain.QueueSnapshot{}, fmt.Errorf("fila: listar: %w", err)
	}
	return domain.QueueSnapshot{Items: items, Preferences: preferences}, nil
}

// EnqueueVideo insere ou recoloca um vídeo no final da fila do perfil ativo.
func (r *Repository) EnqueueVideo(ctx context.Context, videoID string) (domain.QueueItem, error) {
	videoID = strings.TrimSpace(videoID)
	if videoID == "" {
		return domain.QueueItem{}, errors.New("fila: vídeo vazio")
	}
	profileID, err := r.ActiveProfileID(ctx)
	if err != nil {
		return domain.QueueItem{}, err
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return domain.QueueItem{}, fmt.Errorf("fila: iniciar: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	var videoExists int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM videos WHERE id = ?`, videoID).Scan(&videoExists); err != nil {
		return domain.QueueItem{}, fmt.Errorf("fila: validar vídeo: %w", err)
	}
	if videoExists == 0 {
		return domain.QueueItem{}, errors.New("fila: vídeo ainda não existe no catálogo")
	}
	var count int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM playback_queue WHERE profile_id = ?`, profileID).Scan(&count); err != nil {
		return domain.QueueItem{}, fmt.Errorf("fila: contar: %w", err)
	}
	var existingID string
	existingErr := tx.QueryRowContext(ctx, `
		SELECT id FROM playback_queue WHERE profile_id = ? AND video_id = ?`,
		profileID, videoID).Scan(&existingID)
	if existingErr != nil && !errors.Is(existingErr, sql.ErrNoRows) {
		return domain.QueueItem{}, fmt.Errorf("fila: consultar duplicado: %w", existingErr)
	}
	if count >= maxQueueItems && errors.Is(existingErr, sql.ErrNoRows) {
		return domain.QueueItem{}, fmt.Errorf("fila: limite de %d itens atingido", maxQueueItems)
	}

	var nextPosition int
	if err := tx.QueryRowContext(ctx, `
		SELECT COALESCE(MAX(position), -1) + 1 FROM playback_queue WHERE profile_id = ?`,
		profileID).Scan(&nextPosition); err != nil {
		return domain.QueueItem{}, fmt.Errorf("fila: calcular posição: %w", err)
	}
	now := fmtTime(time.Now())
	itemID := existingID
	if errors.Is(existingErr, sql.ErrNoRows) {
		itemID, err = queueItemID()
		if err != nil {
			return domain.QueueItem{}, err
		}
		_, err = tx.ExecContext(ctx, `
			INSERT INTO playback_queue (id, profile_id, video_id, position, state, added_at, played_at)
			VALUES (?, ?, ?, ?, 'pending', ?, '')`, itemID, profileID, videoID, nextPosition, now)
	} else {
		_, err = tx.ExecContext(ctx, `
			UPDATE playback_queue
			SET position = ?, state = 'pending', played_at = ''
			WHERE profile_id = ? AND id = ?`, nextPosition, profileID, itemID)
	}
	if err != nil {
		return domain.QueueItem{}, fmt.Errorf("fila: adicionar: %w", err)
	}
	if err := normalizeQueuePositions(ctx, tx, profileID); err != nil {
		return domain.QueueItem{}, err
	}
	if err := tx.Commit(); err != nil {
		return domain.QueueItem{}, fmt.Errorf("fila: confirmar: %w", err)
	}
	snapshot, err := r.PlaybackQueue(ctx)
	if err != nil {
		return domain.QueueItem{}, err
	}
	for _, item := range snapshot.Items {
		if item.ID == itemID {
			return item, nil
		}
	}
	return domain.QueueItem{}, errors.New("fila: item adicionado não foi encontrado")
}

// ReorderPlaybackQueue aplica uma ordem completa e rejeita listas parciais.
func (r *Repository) ReorderPlaybackQueue(ctx context.Context, itemIDs []string) error {
	profileID, err := r.ActiveProfileID(ctx)
	if err != nil {
		return err
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("fila: iniciar reordenação: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	var count int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM playback_queue WHERE profile_id = ?`, profileID).Scan(&count); err != nil {
		return fmt.Errorf("fila: contar: %w", err)
	}
	if len(itemIDs) != count {
		return errors.New("fila: a nova ordem precisa conter todos os itens")
	}
	seen := make(map[string]struct{}, len(itemIDs))
	for position, itemID := range itemIDs {
		if _, duplicate := seen[itemID]; duplicate || strings.TrimSpace(itemID) == "" {
			return errors.New("fila: ordem contém item vazio ou duplicado")
		}
		seen[itemID] = struct{}{}
		result, err := tx.ExecContext(ctx, `
			UPDATE playback_queue SET position = ? WHERE profile_id = ? AND id = ?`,
			position, profileID, itemID)
		if err != nil {
			return fmt.Errorf("fila: reordenar: %w", err)
		}
		if affected, _ := result.RowsAffected(); affected != 1 {
			return errors.New("fila: ordem contém item inexistente")
		}
	}
	return tx.Commit()
}

func (r *Repository) RemoveQueueItem(ctx context.Context, itemID string) error {
	profileID, err := r.ActiveProfileID(ctx)
	if err != nil {
		return err
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("fila: iniciar remoção: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx, `DELETE FROM playback_queue WHERE profile_id = ? AND id = ?`, profileID, itemID); err != nil {
		return fmt.Errorf("fila: remover: %w", err)
	}
	if err := normalizeQueuePositions(ctx, tx, profileID); err != nil {
		return err
	}
	return tx.Commit()
}

func (r *Repository) MarkQueueItemPlayed(ctx context.Context, itemID string, played bool) error {
	profileID, err := r.ActiveProfileID(ctx)
	if err != nil {
		return err
	}
	preferences, err := r.QueuePreferences(ctx)
	if err != nil {
		return err
	}
	if played && preferences.RemovePlayed {
		return r.RemoveQueueItem(ctx, itemID)
	}
	state := domain.QueueItemPending
	playedAt := ""
	if played {
		state = domain.QueueItemPlayed
		playedAt = fmtTime(time.Now())
	}
	result, err := r.db.ExecContext(ctx, `
		UPDATE playback_queue SET state = ?, played_at = ?
		WHERE profile_id = ? AND id = ?`, state, playedAt, profileID, itemID)
	if err != nil {
		return fmt.Errorf("fila: marcar item: %w", err)
	}
	if affected, _ := result.RowsAffected(); affected != 1 {
		return errors.New("fila: item não encontrado")
	}
	return nil
}

// ClearPlaybackQueue aceita somente all ou played para evitar exclusões amplas acidentais.
func (r *Repository) ClearPlaybackQueue(ctx context.Context, scope string) error {
	profileID, err := r.ActiveProfileID(ctx)
	if err != nil {
		return err
	}
	switch scope {
	case "all":
		_, err = r.db.ExecContext(ctx, `DELETE FROM playback_queue WHERE profile_id = ?`, profileID)
	case "played":
		_, err = r.db.ExecContext(ctx, `DELETE FROM playback_queue WHERE profile_id = ? AND state = 'played'`, profileID)
	default:
		return errors.New("fila: escopo de limpeza inválido")
	}
	if err != nil {
		return fmt.Errorf("fila: limpar: %w", err)
	}
	return r.normalizePlaybackQueue(ctx, profileID)
}

func (r *Repository) QueuePreferences(ctx context.Context) (domain.QueuePreferences, error) {
	profileID, err := r.ActiveProfileID(ctx)
	if err != nil {
		return domain.QueuePreferences{}, err
	}
	preferences := domain.QueuePreferences{Autoplay: true}
	var autoplay, removePlayed int
	err = r.db.QueryRowContext(ctx, `
		SELECT autoplay, remove_played FROM playback_queue_preferences WHERE profile_id = ?`,
		profileID).Scan(&autoplay, &removePlayed)
	if errors.Is(err, sql.ErrNoRows) {
		return preferences, nil
	}
	if err != nil {
		return domain.QueuePreferences{}, fmt.Errorf("fila: carregar preferências: %w", err)
	}
	preferences.Autoplay = autoplay != 0
	preferences.RemovePlayed = removePlayed != 0
	return preferences, nil
}

func (r *Repository) SaveQueuePreferences(ctx context.Context, preferences domain.QueuePreferences) error {
	profileID, err := r.ActiveProfileID(ctx)
	if err != nil {
		return err
	}
	_, err = r.db.ExecContext(ctx, `
		INSERT INTO playback_queue_preferences (profile_id, autoplay, remove_played)
		VALUES (?, ?, ?)
		ON CONFLICT(profile_id) DO UPDATE SET
			autoplay = excluded.autoplay,
			remove_played = excluded.remove_played`,
		profileID, boolInt(preferences.Autoplay), boolInt(preferences.RemovePlayed))
	if err != nil {
		return fmt.Errorf("fila: salvar preferências: %w", err)
	}
	return nil
}

func (r *Repository) SavePlaybackQueueAsPlaylist(ctx context.Context, name string) (domain.Playlist, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return domain.Playlist{}, errors.New("fila: nome da playlist vazio")
	}
	snapshot, err := r.PlaybackQueue(ctx)
	if err != nil {
		return domain.Playlist{}, err
	}
	if len(snapshot.Items) == 0 {
		return domain.Playlist{}, errors.New("fila: não há itens para salvar")
	}
	playlistID, err := PlaylistID()
	if err != nil {
		return domain.Playlist{}, err
	}
	videoIDs := make([]string, 0, len(snapshot.Items))
	for _, item := range snapshot.Items {
		videoIDs = append(videoIDs, item.Video.ID)
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return domain.Playlist{}, fmt.Errorf("fila: iniciar playlist: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	now := fmtTime(time.Now())
	const description = "Criada a partir da fila de reprodução"
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO playlists (id, name, description, color, created_at, updated_at)
		VALUES (?, ?, ?, '', ?, ?)`, playlistID, name, description, now, now); err != nil {
		return domain.Playlist{}, fmt.Errorf("fila: criar playlist: %w", err)
	}
	if err := addPlaylistItemsTx(ctx, tx, playlistID, videoIDs); err != nil {
		return domain.Playlist{}, err
	}
	if err := tx.Commit(); err != nil {
		return domain.Playlist{}, fmt.Errorf("fila: confirmar playlist: %w", err)
	}
	return domain.Playlist{
		ID: playlistID, Name: name, Description: description,
		CreatedAt: parseTime(now), UpdatedAt: parseTime(now), ItemCount: len(videoIDs),
	}, nil
}

func (r *Repository) normalizePlaybackQueue(ctx context.Context, profileID string) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("fila: iniciar normalização: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	if err := normalizeQueuePositions(ctx, tx, profileID); err != nil {
		return err
	}
	return tx.Commit()
}

func normalizeQueuePositions(ctx context.Context, tx *sql.Tx, profileID string) error {
	rows, err := tx.QueryContext(ctx, `
		SELECT id FROM playback_queue WHERE profile_id = ? ORDER BY position, id`, profileID)
	if err != nil {
		return fmt.Errorf("fila: normalizar: %w", err)
	}
	ids := make([]string, 0)
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return fmt.Errorf("fila: normalizar: %w", err)
		}
		ids = append(ids, id)
	}
	if err := rows.Close(); err != nil {
		return fmt.Errorf("fila: normalizar: %w", err)
	}
	for position, id := range ids {
		if _, err := tx.ExecContext(ctx, `
			UPDATE playback_queue SET position = ? WHERE profile_id = ? AND id = ?`,
			position, profileID, id); err != nil {
			return fmt.Errorf("fila: normalizar: %w", err)
		}
	}
	return nil
}
