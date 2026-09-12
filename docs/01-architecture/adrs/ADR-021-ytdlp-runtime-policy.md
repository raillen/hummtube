# ADR-021: Política de Runtime do yt-dlp do Sistema

## Contexto
Extratores quebram com frequência quando o YouTube altera assinaturas e tokens. Embutir versões antigas no app causa falhas silenciosas.

## Decisão
O NanoTube Web utiliza prioritariamente o `yt-dlp` instalado no sistema operacional do usuário, exibindo no painel de diagnósticos avisos caso o binário esteja desatualizado há mais de 6 meses.
