---
id: ui-ux
status: canonical
---

# UI/UX & Design System — NanoTube Web

## 🎨 Princípios de Design

1. **Content-First**: O conteúdo (vídeos, canais, playlists) é o protagonista absoluto;
2. **Minimalismo e Fluidez**: Sem animações lentas, sem anúncios ou elementos intrusivos;
3. **Temas Consistentes**:
   - **Dark Theme** (Padrão): Fundo grafite/preto profundo (`#09090b` / `zinc-950`), texto de alto contraste, destaque em vermelho rubi YouTube (`#ef4444` / `red-500`);
   - **Light Theme**: Fundo cinza frio suave (`#eef1f4`) e superfícies `#f7f8fa`, reduzindo brilho sem perder contraste;
   - **10-Foot TV Theme**: Anel de foco universal ciano (`#22d3ee`, 3px com offset 3px e leve elevação `scale(1.03)`), layout leanback com grade respirável (1–2–3–4 colunas por breakpoint) e sidebar compactada.
4. **Ícones Semânticos**: Exclusivamente **Lucide Icons** (`lucide-svelte`) para consistência visual.

---

## 📐 Anatomia da Interface

```text
┌────────────────────────────────────────────────────────────────────────┐
│ Header: Logo • Barra de Busca Instantânea • Botão Atualizar • Perfil/TV │
├──────────────┬─────────────────────────────────────────────────────────┤
│ Sidebar      │ Content Area                                            │
│ • Home       │ ┌─────────────────────────────────────────────────────┐ │
│ • Inscrições │ │ Chips: data | conteúdo | categoria | assistido     │ │
│ • Canais     │ ├─────────────────────────────────────────────────────┤ │
│ • Busca/Fila │ │ Video Grid (grade/lista/compacto configuráveis)     │ │
│ • Biblioteca │ │ [Card 1]  [Card 2]  [Card 3]  [Carregar mais]      │ │
│ • Ajustes    │ └─────────────────────────────────────────────────────┘ │
├──────────────┴─────────────────────────────────────────────────────────┤
│ MiniPlayer / Player Bar (quando em reprodução):                        │
│ [Thumb] Título • Canal | [Play] [Seek Bar] [Vol/DSP] [Speed] [Full/TV] │
└────────────────────────────────────────────────────────────────────────┘
```

**Inscrições** é o feed de vídeos por data; **Gerenciar canais** é o workspace de seleção em lote, miniaturas, tags, categorias, favoritos, pastas e desinscrição. **Fila** possui página própria para ordenar e salvar sequências. NanoMusic e NanoIPTV são aplicativos separados.

A sidebar pode ser recolhida e a escolha persiste. Ajustes são separados por abas de contas, aparência, conteúdo, sistema e segurança. Busca, inscrições e canais aplicam filtros imediatamente; busca e inscrições restauram os filtros persistidos. Grade, lista e compacto estão disponíveis em playlists, biblioteca, canais e seus detalhes.
