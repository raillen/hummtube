# ADR-001: Go como Linguagem Principal de Backend

## Contexto
O NanoTube Web requer alta performance, concorrência segura com `errgroup`/`singleflight`, baixo consumo de memória e facilidade de compilação cruzada.

## Decisão
Go é mantido como a linguagem exclusiva do backend, dos serviços de catálogo, storage, recomendação e resolução de playback.

## Consequências
- Binário único e leve;
- Integração perfeita com Wails v3;
- Sem garbage collection pesado de runtimes de script no backend.
