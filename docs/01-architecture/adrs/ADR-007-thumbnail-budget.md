# ADR-007: Orçamento e Cache de Miniaturas

## Contexto
O download e renderização indiscriminada de miniaturas de alta resolução causa picos de rede e vazamento de memória.

## Decisão
1. Miniaturas remotas são cacheadas em disco (content-addressed por hash da URL);
2. Carregamento preguiçoso (*lazy loading*) com `IntersectionObserver` no frontend Svelte;
3. Poda periódica de cache com retenção LRU bounded.
