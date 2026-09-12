---
name: wails-svelte-development
description: Guia de desenvolvimento com Wails v3, Svelte 5, Tailwind CSS e Lucide Icons para desktop de alta performance.
---

# Wails v3 + Svelte 5 Development Skill

## Visão Geral
Esta skill orienta a construção de aplicações desktop leves combinando o runtime Wails v3 (Go) com Svelte 5 (Vite SPA), estilização atômica com Tailwind CSS e ícones Lucide.

## Padrões de Implementação

### 1. Comunicação Wails v3 (Go ↔ Svelte)
- Declare serviços em Go com métodos públicos exportados;
- Injete os serviços no `application.New(application.Options{ Services: [...] })`;
- No Svelte, importe as chamadas tipadas geradas em `frontend/src/lib/wailsjs/` ou use os bindings declarados.

### 2. Estado Reativo em Svelte
- Utilize Svelte Runes (`$state`, `$derived`, `$effect`) ou stores reativas tipadas;
- Mantenha o estado do player isolado em um store global (`playerStore.ts`) para suportar transição fluida entre modo minimizado e tela cheia.

### 3. Tailwind CSS & Temas
- Use variáveis CSS semânticas (`bg-background`, `text-foreground`, `border-border`, `accent-primary`) para suportar alternância instantânea entre modo escuro, claro e Modo TV.
