// Playlists locais (M7/PLY-01). Estado puramente local: nada é sincronizado
// com o YouTube (docs/03-implementation/STORAGE.md).
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

// PlaylistID gera um id curto e estável para uma playlist nova.
func PlaylistID() (string, error) {
	buf := make([]byte, 6)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("playlists: gerar id: %w", err)
	}
	return "pl_" + hex.EncodeToString(buf), nil
}

// CreatePlaylist cria uma playlist vazia e devolve a entidade persistida.
func (r *Repository) CreatePlaylist(ctx context.Context, name, description, color string) (domain.Playlist, error) {
	if strings.TrimSpace(name) == "" {
		return domain.Playlist{}, errors.New("playlists: nome vazio")
	}
	id, err := PlaylistID()
	if err != nil {
		return domain.Playlist{}, err
	}
	now := fmtTime(time.Now())
	_, err = r.db.ExecContext(ctx, `
		INSERT INTO playlists (id, name, description, color, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)`,
		id, strings.TrimSpace(name), strings.TrimSpace(description), strings.TrimSpace(color), now, now)
	if err != nil {
		return domain.Playlist{}, fmt.Errorf("playlists: criar: %w", err)
	}
	return domain.Playlist{
		ID: id, Name: strings.TrimSpace(name),
		Description: strings.TrimSpace(description), Color: strings.TrimSpace(color),
		CreatedAt: parseTime(now), UpdatedAt: parseTime(now),
	}, nil
}

// Playlists lista todas as playlists com a contagem de itens, mais recentes
// primeiro (criadas por último ficam no topo da página).
func (r *Repository) Playlists(ctx context.Context) ([]domain.Playlist, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT p.id, p.name, p.description, p.color, p.is_smart, p.created_at, p.updated_at,
		       COUNT(i.video_id)
		FROM playlists p
		LEFT JOIN playlist_items i ON i.playlist_id = p.id
		GROUP BY p.id
		ORDER BY p.created_at DESC, p.id`)
	if err != nil {
		return nil, fmt.Errorf("playlists: listar: %w", err)
	}
	defer rows.Close()
	return scanPlaylistRows(rows, "playlists")
}

// UpdatePlaylist altera nome, descrição e cor. Só campos não vazios mudam, o
// que permite editar um único campo sem sobrescrever os outros.
func (r *Repository) UpdatePlaylist(ctx context.Context, id, name, description, color string) error {
	sets := []string{}
	args := []any{}
	if strings.TrimSpace(name) != "" {
		sets = append(sets, "name = ?")
		args = append(args, strings.TrimSpace(name))
	}
	if strings.TrimSpace(description) != "" {
		sets = append(sets, "description = ?")
		args = append(args, strings.TrimSpace(description))
	}
	if strings.TrimSpace(color) != "" {
		sets = append(sets, "color = ?")
		args = append(args, strings.TrimSpace(color))
	}
	if len(sets) == 0 {
		return nil
	}
	sets = append(sets, "updated_at = ?")
	args = append(args, fmtTime(time.Now()), id)
	query := "UPDATE playlists SET " + strings.Join(sets, ", ") + " WHERE id = ?"
	res, err := r.db.ExecContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("playlists: atualizar: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return errors.New("playlists: playlist não encontrada")
	}
	return nil
}

// DeletePlaylist apaga a playlist; os itens caem pela FK em cascata.
func (r *Repository) DeletePlaylist(ctx context.Context, id string) error {
	if _, err := r.db.ExecContext(ctx, `DELETE FROM playlists WHERE id = ?`, id); err != nil {
		return fmt.Errorf("playlists: excluir: %w", err)
	}
	return nil
}

// DuplicatePlaylist copia playlist e itens na mesma ordem para uma nova.
func (r *Repository) DuplicatePlaylist(ctx context.Context, id, newName string) (domain.Playlist, error) {
	if strings.TrimSpace(newName) == "" {
		return domain.Playlist{}, errors.New("playlists: nome vazio")
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return domain.Playlist{}, fmt.Errorf("playlists: duplicar: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	var src struct {
		ID, Name, Description, Color string
		CreatedAt, UpdatedAt         string
	}
	if err := tx.QueryRowContext(ctx, `
		SELECT id, name, description, color, created_at, updated_at
		FROM playlists WHERE id = ?`, id).Scan(
		&src.ID, &src.Name, &src.Description, &src.Color, &src.CreatedAt, &src.UpdatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.Playlist{}, errors.New("playlists: playlist inexistente")
		}
		return domain.Playlist{}, fmt.Errorf("playlists: duplicar fonte: %w", err)
	}

	newID, err := PlaylistID()
	if err != nil {
		return domain.Playlist{}, err
	}
	now := fmtTime(time.Now())
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO playlists (id, name, description, color, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)`,
		newID, strings.TrimSpace(newName), src.Description, src.Color, now, now); err != nil {
		return domain.Playlist{}, fmt.Errorf("playlists: duplicar inserir: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO playlist_items (playlist_id, video_id, position, added_at)
		SELECT ?, video_id, position, added_at FROM playlist_items WHERE playlist_id = ?`,
		newID, id); err != nil {
		return domain.Playlist{}, fmt.Errorf("playlists: duplicar itens: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return domain.Playlist{}, fmt.Errorf("playlists: duplicar commit: %w", err)
	}
	return domain.Playlist{
		ID: newID, Name: strings.TrimSpace(newName),
		Description: src.Description, Color: src.Color,
		CreatedAt: parseTime(now), UpdatedAt: parseTime(now),
	}, nil
}

// AddPlaylistItem acrescenta um vídeo ao fim da playlist. Reinserir o mesmo
// vídeo não duplica nem muda a ordem.
func (r *Repository) AddPlaylistItem(ctx context.Context, playlistID, videoID string) error {
	if playlistID == "" || videoID == "" {
		return errors.New("playlists: ids vazios")
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("playlists: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	if err := addPlaylistItemTx(ctx, tx, playlistID, videoID); err != nil {
		return err
	}
	return tx.Commit()
}

// addPlaylistItemTx acrescenta um vídeo ao fim da playlist dentro de uma
// transação já aberta. Reinserir o mesmo vídeo não duplica nem reordena.
func addPlaylistItemTx(ctx context.Context, tx *sql.Tx, playlistID, videoID string) error {
	return addPlaylistItemsTx(ctx, tx, playlistID, []string{videoID})
}

// addPlaylistItemsTx acrescenta vários vídeos ao fim da playlist em lote,
// evitando N+1 queries.
func addPlaylistItemsTx(ctx context.Context, tx *sql.Tx, playlistID string, videoIDs []string) error {
	if len(videoIDs) == 0 {
		return nil
	}
	existingRows, err := tx.QueryContext(ctx,
		`SELECT video_id FROM playlist_items WHERE playlist_id = ?`, playlistID)
	if err != nil {
		return fmt.Errorf("playlists: %w", err)
	}
	existing := make(map[string]bool)
	for existingRows.Next() {
		var vid string
		if err := existingRows.Scan(&vid); err == nil {
			existing[vid] = true
		}
	}
	existingRows.Close()

	var next sql.NullInt64
	if err := tx.QueryRowContext(ctx,
		`SELECT MAX(position) FROM playlist_items WHERE playlist_id = ?`, playlistID).Scan(&next); err != nil {
		return fmt.Errorf("playlists: %w", err)
	}
	position := int64(0)
	if next.Valid {
		position = next.Int64 + 1
	}

	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO playlist_items (playlist_id, video_id, position, added_at)
		VALUES (?, ?, ?, ?)`)
	if err != nil {
		return fmt.Errorf("playlists: preparar insercao: %w", err)
	}
	defer stmt.Close()

	now := fmtTime(time.Now())
	for _, id := range videoIDs {
		if existing[id] {
			continue
		}
		if _, err := stmt.ExecContext(ctx, playlistID, id, position, now); err != nil {
			return fmt.Errorf("playlists: %w", err)
		}
		existing[id] = true
		position++
	}
	return nil
}

// CopyPlaylistItems copia vários vídeos para outra playlist, sem remover da
// origem. Duplicados no destino são ignorados; a operação é atômica.
func (r *Repository) CopyPlaylistItems(ctx context.Context, fromID, toID string, videoIDs []string) error {
	if fromID == "" || toID == "" || len(videoIDs) == 0 {
		return errors.New("playlists: ids vazios")
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("playlists: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	if err := addPlaylistItemsTx(ctx, tx, toID, videoIDs); err != nil {
		return fmt.Errorf("playlists: copiar: %w", err)
	}
	return tx.Commit()
}

// MovePlaylistItems move vários vídeos para outra playlist (copiar no destino
// e remover da origem), de forma atômica. Mover para a própria origem não
// muda nada além do custo de reescrever a ordem.
func (r *Repository) MovePlaylistItems(ctx context.Context, fromID, toID string, videoIDs []string) error {
	if fromID == "" || toID == "" || len(videoIDs) == 0 {
		return errors.New("playlists: ids vazios")
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("playlists: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	for _, id := range videoIDs {
		if err := removePlaylistItemTx(ctx, tx, fromID, id); err != nil {
			return fmt.Errorf("playlists: mover: %w", err)
		}
	}
	if err := addPlaylistItemsTx(ctx, tx, toID, videoIDs); err != nil {
		return fmt.Errorf("playlists: mover: %w", err)
	}
	return tx.Commit()
}

// RemovePlaylistItem tira um vídeo da playlist e fecha o buraco de posição,
// para a ordem continuar contígua.
func (r *Repository) RemovePlaylistItem(ctx context.Context, playlistID, videoID string) error {
	if playlistID == "" || videoID == "" {
		return errors.New("playlists: ids vazios")
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("playlists: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	if err := removePlaylistItemTx(ctx, tx, playlistID, videoID); err != nil {
		return err
	}
	return tx.Commit()
}

// removePlaylistItemTx tira um vídeo da playlist dentro de uma transação já
// aberta e fecha o buraco de posição. Remover o que não está lá não é erro.
func removePlaylistItemTx(ctx context.Context, tx *sql.Tx, playlistID, videoID string) error {
	var position int
	err := tx.QueryRowContext(ctx, `
		SELECT position FROM playlist_items
		WHERE playlist_id = ? AND video_id = ?`, playlistID, videoID).Scan(&position)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("playlists: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `
		DELETE FROM playlist_items WHERE playlist_id = ? AND video_id = ?`, playlistID, videoID); err != nil {
		return fmt.Errorf("playlists: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE playlist_items SET position = position - 1
		WHERE playlist_id = ? AND position > ?`, playlistID, position); err != nil {
		return fmt.Errorf("playlists: %w", err)
	}
	return nil
}

// RemovePlaylistItems tira vários vídeos da playlist fechando os buracos de
// posição, em uma única transação.
func (r *Repository) RemovePlaylistItems(ctx context.Context, playlistID string, videoIDs []string) error {
	if playlistID == "" || len(videoIDs) == 0 {
		return errors.New("playlists: ids vazios")
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("playlists: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	for _, id := range videoIDs {
		if err := removePlaylistItemTx(ctx, tx, playlistID, id); err != nil {
			return fmt.Errorf("playlists: remover lote: %w", err)
		}
	}
	return tx.Commit()
}

// PlaylistVideos lista os vídeos da playlist na ordem de posição. Itens cujo
// vídeo sumiu do catálogo são ignorados (o item persiste para reaparecer se o
// vídeo voltar).
func (r *Repository) PlaylistVideos(ctx context.Context, playlistID string) ([]domain.Video, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT `+videoColumns+`
		FROM playlist_items i
		JOIN videos v ON v.id = i.video_id
		WHERE i.playlist_id = ?
		ORDER BY i.position, v.id`, playlistID)
	if err != nil {
		return nil, fmt.Errorf("playlists: vídeos: %w", err)
	}
	defer rows.Close()
	return scanVideoRows(rows, "playlists")
}

// PlaylistVideosFiltered lista os vídeos da playlist aplicando filtros
// combináveis (canal, duração, data, categoria, assistido, favorito, conteúdo e
// Shorts) e uma ordem de exibição. Diferente de PlaylistVideos (ordem de
// posição), esta consulta monta WHERE/ORDER dinamicamente em SQL — os filtros
// rodam antes do LIMIT interno (aceite do PLY-04) — e nunca altera
// playlist_items.position. O padrão é PlaylistVideos; só chame esta quando
// houver filtro ou ordenação ativa.
func (r *Repository) PlaylistVideosFiltered(ctx context.Context, playlistID string, f domain.PlaylistFilter, sort domain.PlaylistSortKey) ([]domain.Video, error) {
	if playlistID == "" {
		return nil, errors.New("playlists: ids vazios")
	}
	query, args, err := playlistVideosFilteredQuery(playlistID, f, sort)
	if err != nil {
		return nil, err
	}
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("playlists: vídeos filtrados: %w", err)
	}
	defer rows.Close()
	return scanVideoRows(rows, "playlists")
}

// playlistVideosFilteredQuery monta a consulta de PlaylistVideosFiltered. Os
// valores do usuário entram sempre como argumentos (?) — nenhum texto do filtro
// é concatenado no SQL.
func playlistVideosFilteredQuery(playlistID string, f domain.PlaylistFilter, sort domain.PlaylistSortKey) (string, []any, error) {
	conds := []string{"i.playlist_id = ?"}
	args := []any{playlistID}
	var joins []string
	channelJoin := false

	if ch := strings.TrimSpace(f.Channel); ch != "" {
		// O filtro de canal compara com o nome (LIKE) — o usuário digita o
		// nome, não o id. O JOIN é feito por baixo, como no card.
		conds = append(conds, `EXISTS (SELECT 1 FROM channels c WHERE c.id = v.channel_id AND c.title LIKE ? ESCAPE '\' COLLATE NOCASE)`)
		args = append(args, escapeLikePattern(ch))
		channelJoin = true
	}

	switch f.Duration {
	case domain.SearchDurationShort:
		conds = append(conds, `v.duration > 0 AND v.duration < 4*60`)
	case domain.SearchDurationMedium:
		conds = append(conds, `v.duration >= 4*60 AND v.duration <= 20*60`)
	case domain.SearchDurationLong:
		conds = append(conds, `v.duration > 20*60`)
	case domain.SearchDurationCustom:
		if f.MinDuration > 0 {
			conds = append(conds, `v.duration >= ?`)
			args = append(args, int64(f.MinDuration/time.Second))
		}
		if f.MaxDuration > 0 {
			conds = append(conds, `v.duration <= ?`)
			args = append(args, int64(f.MaxDuration/time.Second))
		}
	}

	switch f.Age {
	case domain.FeedAgeToday:
		conds = append(conds, `v.published_at >= ?`)
		args = append(args, fmtTime(time.Now().Add(-24*time.Hour)))
	case domain.FeedAgeWeek:
		conds = append(conds, `v.published_at >= ?`)
		args = append(args, fmtTime(time.Now().Add(-7*24*time.Hour)))
	case domain.FeedAgeMonth:
		conds = append(conds, `v.published_at >= ?`)
		args = append(args, fmtTime(time.Now().Add(-30*24*time.Hour)))
	case domain.FeedAgeYear:
		conds = append(conds, `v.published_at >= ?`)
		args = append(args, fmtTime(time.Now().Add(-365*24*time.Hour)))
	}

	if cat := strings.TrimSpace(f.Category); cat != "" {
		conds = append(conds, `v.category = ? COLLATE NOCASE`)
		args = append(args, cat)
	}

	switch f.Watched {
	case domain.SearchWatchWatched:
		// Assistido = completado ou com pelo menos 15s de progresso (mesma
		// semântica da busca local). LEFT JOIN para vídeo sem progresso sair.
		joins = append(joins, "LEFT JOIN playback_progress pp ON pp.video_id = v.id")
		conds = append(conds, `(pp.completed = 1 OR pp.position_ms >= 15000)`)
	case domain.SearchWatchUnwatched:
		joins = append(joins, "LEFT JOIN playback_progress pp ON pp.video_id = v.id")
		conds = append(conds, `(pp.video_id IS NULL OR (pp.completed = 0 AND pp.position_ms < 15000))`)
	case domain.SearchWatchContinue:
		joins = append(joins, "LEFT JOIN playback_progress pp ON pp.video_id = v.id")
		conds = append(conds, `pp.video_id IS NOT NULL AND pp.completed = 0 AND pp.position_ms > 0 AND pp.duration_ms > 0 AND (pp.duration_ms - pp.position_ms) > 30000`)
	}

	if f.Favorite {
		conds = append(conds, `EXISTS (SELECT 1 FROM favorites fav WHERE fav.video_id = v.id)`)
	}

	switch f.Content {
	case domain.FeedRegular:
		conds = append(conds, `NOT (`+liveKindCondition("v.live_status")+`)`)
	case domain.FeedLive:
		conds = append(conds, liveKindCondition("v.live_status"))
	}

	if f.ShortsOnly {
		conds = append(conds, `v.duration > 0 AND v.duration <= 3*60`)
	}

	orderBy, err := playlistSortOrderBy(sort)
	if err != nil {
		return "", nil, err
	}
	// A ordenação por canal precisa do nome do canal; se o filtro de canal não
	// forçou o EXISTS, adiciona um JOIN simples.
	if sort == domain.PlaylistSortChannel && !channelJoin {
		joins = append(joins, "JOIN channels c ON c.id = v.channel_id")
	}
	joinClause := strings.Join(joins, "\n")
	args = append(args, domain.PlaylistFilterLimit)
	query := `
		SELECT ` + videoColumns + `
		FROM playlist_items i
		JOIN videos v ON v.id = i.video_id
		` + joinClause + `
		WHERE ` + strings.Join(conds, " AND ") + `
		ORDER BY ` + orderBy + `
		LIMIT ?`
	return query, args, nil
}

// playlistSortOrderBy devolve a cláusula ORDER BY de uma chave de ordenação.
// O padrão é a ordem de posição persistida.
func playlistSortOrderBy(sort domain.PlaylistSortKey) (string, error) {
	switch sort {
	case "", domain.PlaylistSortManual:
		return "i.position, v.id", nil
	case domain.PlaylistSortTitle:
		return "v.title COLLATE NOCASE, v.id", nil
	case domain.PlaylistSortChannel:
		return "c.title COLLATE NOCASE, v.id", nil
	case domain.PlaylistSortPublished:
		return "v.published_at DESC, v.id", nil
	case domain.PlaylistSortDuration:
		return "v.duration, v.id", nil
	case domain.PlaylistSortAdded:
		return "i.added_at, i.position, v.id", nil
	default:
		return "", fmt.Errorf("playlists: chave de ordenação inválida %q", sort)
	}
}

// liveKindCondition devolve uma expressão SQL que casa com qualquer estado de
// transmissão (agora, agendada ou encerrada) — o mesmo vocabulário de aliases
// do domínio (LiveKindOf). O campo (coluna) é passado pelo chamador.
func liveKindCondition(col string) string {
	aliases := []string{"live", "is_live", "post_live", "was_live", "completed", "is_upcoming", "upcoming"}
	parts := make([]string, 0, len(aliases))
	for _, a := range aliases {
		parts = append(parts, col+" = '"+a+"'")
	}
	return "(" + strings.Join(parts, " OR ") + ")"
}

// ChannelNames devolve o mapa id → título para os ids informados. É o que a UI
// usa para listar canais no filtro do detalhe da playlist sem carregar a lista
// inteira de vídeos.
func (r *Repository) ChannelNames(ctx context.Context, ids []string) (map[string]string, error) {
	if len(ids) == 0 {
		return map[string]string{}, nil
	}
	placeholders := strings.TrimSuffix(strings.Repeat("?,", len(ids)), ",")
	args := make([]any, 0, len(ids))
	for _, id := range ids {
		args = append(args, id)
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, title FROM channels WHERE id IN (`+placeholders+`)`, args...)
	if err != nil {
		return nil, fmt.Errorf("playlists: canais: %w", err)
	}
	defer rows.Close()
	out := map[string]string{}
	for rows.Next() {
		var id, title string
		if err := rows.Scan(&id, &title); err != nil {
			return nil, fmt.Errorf("playlists: canais scan: %w", err)
		}
		out[id] = title
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("playlists: canais rows: %w", err)
	}
	return out, nil
}

// PlaylistItemCount devolve quantos itens tem a playlist.
func (r *Repository) PlaylistItemCount(ctx context.Context, playlistID string) (int, error) {
	var n int
	if err := r.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM playlist_items WHERE playlist_id = ?`, playlistID).Scan(&n); err != nil {
		return 0, fmt.Errorf("playlists: contar itens: %w", err)
	}
	return n, nil
}

// PlaylistsContaining devolve o conjunto de playlists que já têm o vídeo.
func (r *Repository) PlaylistsContaining(ctx context.Context, videoID string) (map[string]bool, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT playlist_id FROM playlist_items WHERE video_id = ?`, videoID)
	if err != nil {
		return nil, fmt.Errorf("playlists: contendo: %w", err)
	}
	defer rows.Close()
	out := map[string]bool{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("playlists: contendo scan: %w", err)
		}
		out[id] = true
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("playlists: contendo rows: %w", err)
	}
	return out, nil
}

// PlaylistMembership devolve a relação completa vídeo → playlists que o contêm.
// É o cache que a UI usa no submenu "Adicionar à playlist" sem consultar o
// banco por card (docs/03-implementation/UI_IMPLEMENTATION.md, "View/controller").
func (r *Repository) PlaylistMembership(ctx context.Context) (map[string]map[string]bool, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT video_id, playlist_id FROM playlist_items`)
	if err != nil {
		return nil, fmt.Errorf("playlists: membership: %w", err)
	}
	defer rows.Close()
	out := map[string]map[string]bool{}
	for rows.Next() {
		var videoID, playlistID string
		if err := rows.Scan(&videoID, &playlistID); err != nil {
			return nil, fmt.Errorf("playlists: membership scan: %w", err)
		}
		if out[videoID] == nil {
			out[videoID] = map[string]bool{}
		}
		out[videoID][playlistID] = true
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("playlists: membership rows: %w", err)
	}
	return out, nil
}

// MovePlaylistItem move um item de posição (delta ±1 é o uso normal da UI;
// valores maiores movem em blocos). A troca é atômica e preserva a contiguidade.
func (r *Repository) MovePlaylistItem(ctx context.Context, playlistID, videoID string, delta int) error {
	if delta == 0 {
		return nil
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("playlists: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	var position int
	err = tx.QueryRowContext(ctx, `
		SELECT position FROM playlist_items
		WHERE playlist_id = ? AND video_id = ?`, playlistID, videoID).Scan(&position)
	if errors.Is(err, sql.ErrNoRows) {
		return tx.Commit()
	}
	if err != nil {
		return fmt.Errorf("playlists: %w", err)
	}

	var max sql.NullInt64
	if err := tx.QueryRowContext(ctx,
		`SELECT MAX(position) FROM playlist_items WHERE playlist_id = ?`, playlistID).Scan(&max); err != nil {
		return fmt.Errorf("playlists: %w", err)
	}
	bound := 0
	if max.Valid {
		bound = int(max.Int64)
	}

	target := position + delta
	if target < 0 {
		target = 0
	}
	if target > bound {
		target = bound
	}
	if target == position {
		return tx.Commit()
	}

	// Desloca a faixa [target, position) ou (position, target] num passo e
	// coloca o item na posição alvo.
	if target > position {
		if _, err := tx.ExecContext(ctx, `
			UPDATE playlist_items SET position = position - 1
			WHERE playlist_id = ? AND position > ? AND position <= ?`,
			playlistID, position, target); err != nil {
			return fmt.Errorf("playlists: %w", err)
		}
	} else {
		if _, err := tx.ExecContext(ctx, `
			UPDATE playlist_items SET position = position + 1
			WHERE playlist_id = ? AND position >= ? AND position < ?`,
			playlistID, target, position); err != nil {
			return fmt.Errorf("playlists: %w", err)
		}
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE playlist_items SET position = ?
		WHERE playlist_id = ? AND video_id = ?`, target, playlistID, videoID); err != nil {
		return fmt.Errorf("playlists: %w", err)
	}
	return tx.Commit()
}

// MovePlaylistItemToIndex reposiciona um item em um índice absoluto (ordem
// contígua 0..n-1), como o drag-and-drop precisa. Reescreve as posições em
// uma única transação; mover para o mesmo índice é um no-op.
func (r *Repository) MovePlaylistItemToIndex(ctx context.Context, playlistID, videoID string, index int) error {
	if index < 0 {
		index = 0
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("playlists: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	rows, err := tx.QueryContext(ctx, `
		SELECT video_id FROM playlist_items
		WHERE playlist_id = ? ORDER BY position`, playlistID)
	if err != nil {
		return fmt.Errorf("playlists: %w", err)
	}
	var order []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return fmt.Errorf("playlists: %w", err)
		}
		order = append(order, id)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return fmt.Errorf("playlists: %w", err)
	}

	from := -1
	for i, id := range order {
		if id == videoID {
			from = i
			break
		}
	}
	if from < 0 {
		// Item fora da playlist: nada a reordenar.
		return tx.Commit()
	}
	if index > len(order)-1 {
		index = len(order) - 1
	}
	if index == from {
		return tx.Commit()
	}

	order = append(order[:from], order[from+1:]...)
	order = append(order, "")
	copy(order[index+1:], order[index:])
	order[index] = videoID

	stmt, err := tx.PrepareContext(ctx, `
		UPDATE playlist_items SET position = ?
		WHERE playlist_id = ? AND video_id = ?`)
	if err != nil {
		return fmt.Errorf("playlists: %w", err)
	}
	defer stmt.Close()
	for i, id := range order {
		if _, err := stmt.ExecContext(ctx, i, playlistID, id); err != nil {
			return fmt.Errorf("playlists: %w", err)
		}
	}
	return tx.Commit()
}

// scanPlaylistRows decodes playlist rows selected by Playlists.
func scanPlaylistRows(rows *sql.Rows, what string) ([]domain.Playlist, error) {
	var out []domain.Playlist
	for rows.Next() {
		var (
			p         domain.Playlist
			created   string
			updated   string
			isSmart   int
			itemCount int
		)
		if err := rows.Scan(&p.ID, &p.Name, &p.Description, &p.Color, &isSmart, &created, &updated, &itemCount); err != nil {
			return nil, fmt.Errorf("%s scan: %w", what, err)
		}
		p.IsSmart = isSmart != 0
		p.CreatedAt = parseTime(created)
		p.UpdatedAt = parseTime(updated)
		p.ItemCount = itemCount
		out = append(out, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("%s rows: %w", what, err)
	}
	return out, nil
}

// Playlist recupera uma única playlist pelo id.
func (r *Repository) Playlist(ctx context.Context, id string) (domain.Playlist, error) {
	var (
		p         domain.Playlist
		created   string
		updated   string
		isSmart   int
		itemCount int
	)
	err := r.db.QueryRowContext(ctx, `
		SELECT p.id, p.name, p.description, p.color, p.is_smart, p.created_at, p.updated_at,
		       COUNT(i.video_id)
		FROM playlists p
		LEFT JOIN playlist_items i ON i.playlist_id = p.id
		WHERE p.id = ?
		GROUP BY p.id`, id).Scan(&p.ID, &p.Name, &p.Description, &p.Color, &isSmart, &created, &updated, &itemCount)
	if err != nil {
		return domain.Playlist{}, fmt.Errorf("playlist %s: %w", id, err)
	}
	p.IsSmart = isSmart != 0
	p.CreatedAt = parseTime(created)
	p.UpdatedAt = parseTime(updated)
	p.ItemCount = itemCount
	return p, nil
}
