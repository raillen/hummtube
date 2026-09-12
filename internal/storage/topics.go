// Temas de interesse: derivação base a partir do histórico assistido e
// ajuste explícito via feedback "mais/menos deste tema"
// (docs/03-implementation/RECOMMENDATIONS.md).
package storage

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/nanotube/nanotube-web/internal/domain"
)

// topicSeedLimit é quantos temas base a derivação cria por vez. O engine só
// usa os 3 mais fortes; 12 dá folga para ajustes do usuário derrubarem alguns.
const topicSeedLimit = 12

// topicSeedMinCount é a frequência mínima de um token no histórico assistido
// para virar tema — evita seções temáticas de palavra avulsa.
const topicSeedMinCount = 2

// RefreshInterestTopics deriva temas base da frequência de tokens em títulos
// e categorias dos vídeos com progresso local e semeia `interest_topics` com
// INSERT OR IGNORE: ajustes manuais (feedback) nunca são sobrescritos.
// Determinístico: empate de frequência resolve por ordem alfabética.
func (r *Repository) RefreshInterestTopics(ctx context.Context) error {
	rows, err := r.db.QueryContext(ctx, `
		SELECT v.title, COALESCE(v.category, '')
		FROM videos v
		JOIN playback_progress pp ON pp.video_id = v.id`)
	if err != nil {
		return fmt.Errorf("interest topics seed: %w", err)
	}
	defer rows.Close()

	counts := make(map[string]int)
	for rows.Next() {
		var title, category string
		if err := rows.Scan(&title, &category); err != nil {
			return fmt.Errorf("interest topics seed scan: %w", err)
		}
		for _, tok := range domain.TopicTokens(title + " " + category) {
			counts[tok]++
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("interest topics seed rows: %w", err)
	}

	type pair struct {
		topic string
		count int
	}
	var pairs []pair
	for topic, count := range counts {
		if count >= topicSeedMinCount {
			pairs = append(pairs, pair{topic, count})
		}
	}
	sort.Slice(pairs, func(i, j int) bool {
		if pairs[i].count != pairs[j].count {
			return pairs[i].count > pairs[j].count
		}
		return pairs[i].topic < pairs[j].topic
	})
	if len(pairs) > topicSeedLimit {
		pairs = pairs[:topicSeedLimit]
	}

	now := fmtTime(time.Now())
	for _, p := range pairs {
		if _, err := r.db.ExecContext(ctx, `
			INSERT INTO interest_topics (topic, score, updated_at) VALUES (?, ?, ?)
			ON CONFLICT(topic) DO NOTHING`, p.topic, p.count, now); err != nil {
			return fmt.Errorf("interest topics seed insert: %w", err)
		}
	}
	return nil
}

// AdjustTopicScore aplica um delta no score de um tema. Tema novo só nasce de
// reforço positivo; score em 0 ou menos remove o tema do perfil.
func (r *Repository) AdjustTopicScore(ctx context.Context, topic string, delta int) error {
	topic = strings.TrimSpace(topic)
	if topic == "" || delta == 0 {
		return fmt.Errorf("interest topics: tema ou delta inválido")
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("interest topics: %w", err)
	}
	defer tx.Rollback()
	if err := applyTopicScoreTx(ctx, tx, topic, delta); err != nil {
		return err
	}
	return tx.Commit()
}

func applyTopicScoreTx(ctx context.Context, tx *sql.Tx, topic string, delta int) error {
	res, err := tx.ExecContext(ctx, `
		UPDATE interest_topics SET score = score + ?, updated_at = ? WHERE topic = ?`,
		delta, fmtTime(time.Now()), topic)
	if err != nil {
		return fmt.Errorf("interest topics: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 && delta > 0 {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO interest_topics (topic, score, updated_at) VALUES (?, ?, ?)`,
			topic, delta, fmtTime(time.Now())); err != nil {
			return fmt.Errorf("interest topics: %w", err)
		}
	}
	if _, err := tx.ExecContext(ctx,
		`DELETE FROM interest_topics WHERE topic = ? AND score <= 0`, topic); err != nil {
		return fmt.Errorf("interest topics: %w", err)
	}
	return nil
}
