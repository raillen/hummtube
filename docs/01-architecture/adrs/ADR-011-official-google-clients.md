# ADR-011: Clientes Oficiais Google para Catálogo

## Contexto
O catálogo de inscrições e metadados de canais requer estabilidade contra quebras de scraping.

## Decisão
Utilizar o SDK oficial `google.golang.org/api/youtube/v3` com cliente autenticado via `golang.org/x/oauth2` para sincronização autoritativa de inscrições e metadados de canais.
