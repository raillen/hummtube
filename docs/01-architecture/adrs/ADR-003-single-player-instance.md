# ADR-003: Instância Única Ativa de Reprodução

## Contexto
Múltiplas instâncias de decodificação de vídeo concorrentes consomem CPU/GPU excessiva e competem por largura de banda.

## Decisão
A aplicação mantém rigorosamente **apenas uma sessão ativa de mídia decodificando por vez**. Ao iniciar um novo vídeo, a sessão anterior é pausada/destruída e seu progresso persistido.
