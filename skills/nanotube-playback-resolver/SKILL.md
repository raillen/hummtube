---
name: nanotube-playback-resolver
description: Guia de resolução de mídia, extração yt-dlp, PO Token attestation e cascading fallback.
---

# NanoTube Playback Resolver Skill

## Visão Geral
Esta skill fornece conhecimento especializado sobre a extração resiliente de fluxos de vídeo e áudio do YouTube e serviços compatíveis.

## Estratégias Principais
1. **yt-dlp JSON Mode**: Executar `yt-dlp --dump-json` com clientes móveis (`android`, `ios`) que não sofrem restrições agressivas de PO Token;
2. **PO Token Attestation**: Quando o cliente `mweb` for necessário, integrar via subprocesso com `bgutil-ytdlp-pot-provider` / Deno;
3. **Invidious Fallback**: Fallback resiliente com checagem rigorosa de SSRF contra IPs privados e loopback;
4. **HLS & DASH**: Muxing de faixas adaptativas de vídeo e áudio quando formatos combinados não estiverem disponíveis.
