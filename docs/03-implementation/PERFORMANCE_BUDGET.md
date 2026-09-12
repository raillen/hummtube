---
id: performance-budget
status: canonical
---

# Orçamento de Performance — NanoTube Web

## Metas em Hardware de Referência (2 GB RAM / Dual-Core)

| Métrica | Orçamento Máximo |
|---|---|
| Tempo de Inicialização (Cold Start) | < 1.5 s |
| Consumo de RAM em Repouso | < 180 MB |
| Consumo de RAM em Playback (1080p) | < 350 MB |
| Uso de CPU em Repouso | < 1.0 % |
| Tamanho do Bundle Frontend (Gzip) | < 150 KB |
| Latência da Busca Instantânea Local | < 15 ms |

## Medição atual do frontend

O build de 2026-08-30 separa as views por rota, importa os ícones individualmente e carrega o decoder HLS sob demanda. O chunk inicial caiu de aproximadamente **1.093 kB / 309 kB gzip** para **205 kB / 53,42 kB gzip**. As views individuais variam de aproximadamente 8 kB a 59 kB antes de gzip; o HLS fica em um chunk separado de **591,72 kB / 185,20 kB gzip**, baixado somente quando um stream `.m3u8` é reproduzido. O orçamento inicial de 150 kB gzip foi atendido para a entrada; o aviso do Vite permanece apenas para o chunk opcional do HLS.
