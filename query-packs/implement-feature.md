# Query Pack: Implement Feature

1. Ler `ATLAS.md` e os documentos de requisitos e arquitetura afetados;
2. Verificar ADRs existentes no `DECISION_REGISTER.md`;
3. Definir contratos de backend Go (`internal/domain`) e serviços Wails (`internal/services`);
4. Implementar repositórios SQLite e migrações se necessário (`internal/storage`);
5. Construir ou atualizar componentes Svelte (`frontend/src/lib/components`);
6. Adicionar testes unitários e de integração (`task test`);
7. Executar gate de verificação: `task verify`;
8. Atualizar documentação canônica.
