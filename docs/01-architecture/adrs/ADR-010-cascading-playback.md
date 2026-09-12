# ADR-010: Cascading Playback Resolver

## Contexto
O YouTube bloqueia frequentemente clientes que não enviam attestation adequada ou tokens PO válidos.

## Decisão
Implementar o `CascadingResolver` que encadeia ordenadamente:
1. `ExplicitYtDlpResolver` com PO Token Attestation (`android`, `ios`, `mweb`);
2. `InvidiousResolver` como fallback transparente;
3. `DirectStreamResolver` para URLs de vídeo direto ou IPTV.

## Consequências
Resiliência máxima contra mudanças e bloqueios sem intervenção do usuário.
