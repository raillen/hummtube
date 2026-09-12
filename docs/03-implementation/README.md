# Guias Técnicos de Implementação (`docs/03-implementation/`)

## O que é este diretório?
Detalhamento técnico aprofundado dos subsistemas de software do NanoTube Web.

## Para que serve?
Serve como referência canônica para construção, manutenção e refatoração de código.

## Inventário
- `STACK.md`: Stack tecnológica backend e frontend;
- `PLAYER.md`: Implementação do player Web/Wails e controles de mídia;
- `PLAYBACK_BACKENDS.md`: Estratégias do CascadingResolver (yt-dlp, PO Token, Invidious);
- `YOUTUBE_PLAYBACK_MODERNIZATION.md`: PO Token Attestation e arquitetura de solvers;
- `STORAGE.md`: SQLite, FTS5 e migrações versionadas Goose;
- `SEARCH.md`: Busca híbrida instantânea;
- `RECOMMENDATIONS.md`: Algoritmo MMR v2 e saturação de canais;
- `THUMBNAILS_AND_CACHE.md`: Gestão de memória e cache em disco;
- `CONCURRENCY.md`: Padrões de concorrência com errgroup e singleflight;
- `BUILD_AND_TOOLING.md`: Configurações de build e Taskfile;
- `DEPENDENCY_POLICY.md`: Política estrita de dependências;
- `PERFORMANCE_BUDGET.md`: Metas de latência e consumo de RAM.
