package iptv

import (
	"context"
	"fmt"
	"time"
)

// CatalogStore é a fronteira mínima entre ingestão IPTV e persistência. A
// implementação concreta pode ser SQLite, fixture de teste ou outro store.
type CatalogStore interface {
	ReplaceIPTVCatalog(context.Context, SourceConfig, Playlist, time.Time) error
}

// CatalogImporter é o caminho de persistência em lotes: recebe um yield por
// lote e grava dentro da própria transação, sem acumular a playlist.
// Implementações concretas (SQLite) preferem este caminho para listas
// grandes; a memória fica limitada ao tamanho do lote.
type CatalogImporter interface {
	ImportIPTVCatalog(context.Context, SourceConfig, time.Time, func(func([]Item) error) error) error
}

// streamImportLimit acompanha playlists reais acima do antigo teto de
// 100 mil entradas (caso observado: ~644 mil). A memória permanece limitada
// porque os itens chegam em lotes e vão direto para a transação.
const streamImportLimit = 1_500_000

// streamImportBatchSize equilibra overhead de INSERT com granularidade de
// progresso; 5 mil itens ≈ alguns MB por lote.
const streamImportBatchSize = 5_000

// CatalogSync coordena uma importação M3U sem conhecer GTK, SQL ou CalmTV.
type CatalogSync struct {
	Source HTTPM3USource
	Store  CatalogStore
	Now    func() time.Time
}

func (sync CatalogSync) SyncM3U(ctx context.Context, config SourceConfig) (Playlist, error) {
	if sync.Store == nil {
		return Playlist{}, fmt.Errorf("sincronizar IPTV: store nil")
	}
	now := time.Now
	if sync.Now != nil {
		now = sync.Now
	}
	stamp := now().UTC()

	// Caminho preferencial: persistência em lotes dentro de uma transação.
	// A playlist nunca é acumulada em memória, então listas enormes
	// (~644 mil entradas reais) importam sem estourar o teto antigo.
	if importer, ok := sync.Store.(CatalogImporter); ok && importer != nil {
		var warnings []ParseWarning
		streamErr := importer.ImportIPTVCatalog(ctx, config, stamp, func(yield func([]Item) error) error {
			summary, err := sync.Source.FetchStream(ctx, config, ParseOptions{
				SourceID:   config.ID,
				SourceName: config.Name,
				MaxItems:   streamImportLimit,
				BatchSize:  streamImportBatchSize,
			}, yield)
			if err != nil {
				return err
			}
			warnings = summary.Warnings
			return nil
		})
		if streamErr != nil {
			return Playlist{}, fmt.Errorf("sincronizar IPTV: persistir: %w", streamErr)
		}
		return Playlist{Warnings: warnings}, nil
	}

	// Fallback para stores que só conhecem o snapshot completo.
	playlist, err := sync.Source.Fetch(ctx, config)
	if err != nil {
		return Playlist{}, fmt.Errorf("sincronizar IPTV: fetch: %w", err)
	}
	if err := sync.Store.ReplaceIPTVCatalog(ctx, config, playlist, stamp); err != nil {
		return playlist, fmt.Errorf("sincronizar IPTV: persistir: %w", err)
	}
	return playlist, nil
}
