---
id: accessibility
status: canonical
---

# Acessibilidade e Navegação 10-Foot — NanoTube Web

## 1. Diretrizes de Acessibilidade
- **Contraste de Cores**: WCAG AAA em elementos de texto e badges;
- **ARIA Labels**: Todos os botões interativos e sliders possuem `aria-label` e `role` semânticos;
- **Navegação Completa por Teclado**: Sem armadilhas de foco (*focus traps*) indesejadas.

## 2. Navegação Espacial no Modo TV
- O Modo TV ativa um motor de navegação 2D (`SpatialNav.ts`):
  - Setas / D-Pad movem o foco bidimensionalmente entre cards e menus, exceto em campos de texto, menus abertos, sliders e mídia;
  - `Enter` ou botão de seleção confirma controles `.tv-focusable` não nativos (controles nativos usam o comportamento padrão);
  - Modais fecham com `Escape` e devolvem o foco ao gatilho; fora de modais, `Escape`/`BrowserBack` volta no histórico;
  - Modo TV persiste em `ui.tv_mode` (backend), `?tv=1` e `localStorage`;
  - O botão de mover o miniplayer tem classe `.tv-focusable`, mantém o foco após mudar de canto e anuncia a posição atual/próxima; a posição é global, persistida e não troca o elemento de mídia.
  - Anel de foco com halo luminoso e escala `1.03x` nos elementos ativos (ciano `#22d3ee` de 3px com offset 3px);
  - Layout leanback em `.tv-mode`: grade respirável de até 4 colunas em telas largas, sidebar compactada sem rótulos ou cartão de hardware e botões de ação ampliados (≥44px / 2.75rem);
  - Seek bar e volume do player com altura e alvos maiores para facilitar uso com D-Pad/gamepad.

O motor usa o foco real do DOM como fonte de verdade, ignora elementos ocultos e limita a navegação ao diálogo ativo. Gamepads mapeiam D-Pad/analógico para direção, botão A para ativar e B para voltar/fechar. `prefers-reduced-motion` elimina transições não essenciais.

Menus contextuais abrem por botão direito ou `Shift+F10`, movem o foco com setas e o devolvem ao gatilho ao fechar com `Escape`.
