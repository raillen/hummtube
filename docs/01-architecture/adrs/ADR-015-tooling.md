# ADR-015: Padronização de Tooling com Taskfile

## Contexto
O processo de build, lint, teste e empacotamento deve ser unificado e reprodutível.

## Decisão
Adotar o `Taskfile.yml` (`go-task`) como orquestrador padrão para todos os comandos de desenvolvimento, gates de verificação (`task verify`), testes e empacotamento.
