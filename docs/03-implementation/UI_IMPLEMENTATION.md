---
id: ui-implementation
status: canonical
---

# Implementação da UI (Svelte + Tailwind CSS + Lucide)

## 1. Princípios da Camada de Visualização
- **Sem DOM Virtual**: O Svelte compila componentes diretamente em código JavaScript cirúrgico;
- **Estilização com Tailwind CSS**: Classes de utilitário sem CSS global disperso, garantindo consistência com o tema configurado;
- **Ícones**: Lucide Icons importados sob demanda de `lucide-svelte`.

## 2. Estrutura de Componentes

### Componentes de Layout
- `Header.svelte`: Barra de pesquisa, botão atualizar, seletor de tema, status de sincronização e botão Modo TV;
- `Sidebar.svelte`: Navegação do NanoTube entre Home, Inscrições, Canais, Busca, Playlists, Fila, Biblioteca e Configurações;
- `MiniPlayer.svelte`: Barra inferior persistente quando o usuário navega enquanto assiste.

### Componentes de Visualização (Views)
- `HomeView.svelte`: Seções do feed local ("Para Você", tópicos em carrossel, chips de filtro rápido);
- `ChannelsView.svelte`: Gestão de pastas de inscrições, canais favoritos e lista paginada;
- `SearchView.svelte`: Busca instantânea FTS5 local + resultados remotos com filtros avançados;
- `ChannelDetailView.svelte`: Página de canal com vídeos públicos, playlists oficiais, paginação, fila e inscrição local;
- `PlaylistsView.svelte` & `PlaylistDetailView.svelte`: Criação, drag-and-drop de faixas, editor de regras inteligentes;
- `LibraryView.svelte`: Histórico com busca e botão de limpar, favoritos, notas e bookmarks salvos;
- `apps/nanoiptv/App.svelte` + `IPTVView.svelte`: raiz independente com listas M3U, categorias e EPG;
- `apps/nanomusic/App.svelte`: raiz independente com descoberta musical, fila, playlists, biblioteca e Last.fm;
- `SettingsView.svelte`: Gestão de conta OAuth, aparência, atalhos de teclado, exportação/importação de dados e diagnósticos.

## 3. Carregamento de módulos

O Vite seleciona uma raiz por `NANOSUITE_APP`. `App.svelte` é exclusivo do NanoTube e carrega suas views por `import()`; NanoIPTV e NanoMusic possuem raízes explícitas sob `src/apps/`. Hooks de persistência/scrobble do player também são resolvidos por produto para impedir dependências de domínio no bundle errado.

## 4. Catálogo de textos e tradução

O frontend possui um catálogo incremental em `frontend/src/locales/` e o
adaptador `frontend/src/lib/i18n/index.ts`. Componentes novos devem usar
`t('namespace.chave')` em vez de fixar textos de domínio no código. O idioma
selecionado é persistido pela configuração `locale`, refletido em
`document.documentElement.lang` e usa `pt-BR` como fallback seguro. A checagem
de paridade pode usar `catalogKeys` e `missingCatalogKeys` antes de publicar um
novo idioma.
