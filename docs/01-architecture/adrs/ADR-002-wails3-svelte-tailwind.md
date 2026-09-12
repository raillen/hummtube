# ADR-002: Adoção de Wails v3 + Svelte + Tailwind CSS + Lucide Icons

## Contexto
O cliente GTK3 original possuía restrições em componentes visuais complexos, animações fluidas e suporte a temas/10-foot UI cross-platform. Frameworks baseados em Electron possuem overhead excessivo de memória (~400MB+).

## Decisão
Adotar **Wails v3** como framework desktop, com **Svelte 5** como camada declarativa de frontend SPA, **Tailwind CSS** para estilização semântica e **Lucide Icons** (`lucide-svelte`) para ícones consistentes.

## Consequências
- Uso de memória reduzido para ~100-180 MB em repouso graças ao WebView nativo do SO;
- UI reativa com feedback instantâneo;
- Desenvolvimento moderno com HMR via Vite e tipagem TypeScript estrita nos contratos.
