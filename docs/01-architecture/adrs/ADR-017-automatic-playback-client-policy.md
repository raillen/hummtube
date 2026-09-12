# ADR-017: Política Automática de Seleção de Cliente Playback

## Contexto
Diferentes clientes YouTube possuem diferentes exigências de assinatura e formatos de mídia.

## Decisão
A política padrão seleciona automaticamente:
1. `android` / `ios` como primeira tentativa (menor taxa de bloqueio e links de áudio/vídeo estáveis);
2. `mweb` com PO Token como fallback se `android` falhar;
3. `Invidious` como fallback final.
