package storage

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/nanotube/nanotube-web/internal/domain"
)

// BenchmarkPlaylistVideosFiltered mede a consulta filtrada de uma playlist com
// 10 mil itens (aceite do PLY-04). Referência para o gate M8 no Celeron 1037U:
// rodar `task bench` e comparar com benchstat.
func BenchmarkPlaylistVideosFiltered(b *testing.B) {
	db, err := OpenMemory(context.Background())
	if err != nil {
		b.Fatal(err)
	}
	defer db.Close()
	if err := Migrate(db); err != nil {
		b.Fatal(err)
	}
	r := &Repository{db: db}
	ctx := context.Background()

	// 10 canais, 1.000 vídeos cada → 10 mil itens na playlist.
	const videosPerChannel = 1000
	now := time.Now()
	for c := 0; c < 10; c++ {
		chID := fmt.Sprintf("c%d", c)
		if _, err := db.Exec(`INSERT INTO channels (id, title) VALUES (?, ?)`, chID, "Canal "+chID); err != nil {
			b.Fatal(err)
		}
	}
	p, err := r.CreatePlaylist(ctx, "Bench", "", "")
	if err != nil {
		b.Fatal(err)
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		b.Fatal(err)
	}
	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO videos (id, channel_id, title, category, duration, live_status, published_at, first_seen_at, last_seen_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		b.Fatal(err)
	}
	addItem, err := tx.PrepareContext(ctx, `
		INSERT INTO playlist_items (playlist_id, video_id, position, added_at)
		VALUES (?, ?, ?, ?)`)
	if err != nil {
		b.Fatal(err)
	}
	ts := fmtTime(now)
	for i := 0; i < videosPerChannel*10; i++ {
		c := i / videosPerChannel
		id := fmt.Sprintf("v%d", i)
		title := fmt.Sprintf("Vídeo %d do canal %d", i, c)
		duration := int64((i%60 + 1) * 60) // 1..60 min
		if _, err := stmt.ExecContext(ctx, id, fmt.Sprintf("c%d", c), title,
			"Educação", duration, "", ts, ts, ts); err != nil {
			b.Fatal(err)
		}
		if _, err := addItem.ExecContext(ctx, p.ID, id, i, ts); err != nil {
			b.Fatal(err)
		}
	}
	if err := tx.Commit(); err != nil {
		b.Fatal(err)
	}

	filter := domain.PlaylistFilter{
		Channel:  "canal c1",
		Duration: domain.SearchDurationMedium,
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		videos, err := r.PlaylistVideosFiltered(ctx, p.ID, filter, domain.PlaylistSortTitle)
		if err != nil {
			b.Fatal(err)
		}
		if len(videos) == 0 {
			b.Fatal("filtro não devolveu nada")
		}
	}
}
