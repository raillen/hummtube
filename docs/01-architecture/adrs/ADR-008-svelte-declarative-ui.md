# ADR-008: UI Declarativa em Svelte e Componentização

## Contexto
O layout precisa ser modular, altamente responsivo, acessível e fácil de manter.

## Decisão
Utilizar **Svelte 5** como padrão de componentização (Views, Modais, Cards, Player), aproveitando sua compilação sem Virtual DOM e reatividade granular com Runes (`$state`, `$derived`, `$effect`).
