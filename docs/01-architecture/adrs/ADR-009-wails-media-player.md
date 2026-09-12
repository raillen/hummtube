# ADR-009: Player Reativo HTML5 / HLS no WebView com Bridge Wails

## Contexto
O player precisa suportar aceleração por hardware do sistema (VA-API/NVDEC/DXVA) de forma transparente através do WebView e suportar streaming progressivo e HLS/DASH.

## Decisão
Implementar o Player de vídeo diretamente em Svelte utilizando elementos `<video>` com **HLS.js** para streams HLS/DASH e bridge Go para stream proxies ou headers de requisição autenticados quando aplicável.

## Consequências
- Aceleração por hardware delegada ao WebView nativo do sistema operacional;
- Controles de UI (seek, volume, DSP, legendas, speed) construídos de forma pura e reativa em Svelte + Lucide Icons sem necessidade de shims CGO ou X11 embeds frágeis.
