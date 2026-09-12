# Query Pack: Performance Regression

1. Medir consumo de memória do processo em repouso e em reprodução;
2. Medir tempo de inicialização (cold start);
3. Verificar tamanho do bundle JavaScript gerado pelo Vite;
4. Inspecionar queries SQLite com `EXPLAIN QUERY PLAN` para detectar table scans;
5. Validar concorrência e leaks de goroutine com `goleak`.
