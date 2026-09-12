---
id: ui-ux-tests
status: canonical
---

# Testes de UI/UX e Acessibilidade — NanoTube Web

## Casos de Teste Chave
1. **Navegação Básica**: Alternância entre abas sem recarregar estado de mídia ativa;
2. **MiniPlayer Persistence**: Navegar para outras abas enquanto vídeo toca no MiniPlayer;
3. **Miniplayer móvel**: Circular pelos quatro cantos, manter a mesma instância de mídia, preservar o foco no botão de mover e restaurar a posição salva ao reabrir;
3. **Navegação Espacial (Modo TV)**: Percorrer cards de vídeo e menus exclusivamente com setas/teclado sem perda de foco; setas não disputam com seek/volume, sliders, menus ou mídia;
4. **Modo TV persistente**: Ativar persiste `ui.tv_mode`, recarrega ligado e usa `?tv=1` como override;
5. **Layout Leanback**: Modo TV ativa grade respirável (`video-grid-cards`, máximo 4 colunas), sidebar compactada (sem rótulos/recursos), botões de ação de cards e player com alvos ≥44px e seek bar/volume ampliados;
5. **Modal e foco**: `Escape` fecha e devolve o foco ao gatilho; `Tab` permanece contido no diálogo;
6. **Confirmação focável TV**: Ações destrutivas (remover do histórico/favoritos, limpar tudo, remover perfil, reverter runtime, excluir playlist) usam `ConfirmDialog` com `role="alertdialog"`, foco inicial no botão de confirmação, `Tab` ciclando entre os botões e `Escape` cancelando; nada deve cair em `window.confirm`;
7. **Login TV**: Em Modo TV o modal de autenticação abre na aba de código do dispositivo, com polling único cancelável;
8. **Tema Claro/Escuro**: Alternar temas e validar contraste sem quebras visuais.
