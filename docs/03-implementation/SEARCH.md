---
id: search
status: canonical
---

# Pesquisa Híbrida e Filtros — NanoTube Web

## Como funciona

A digitação consulta somente o FTS5 local, com debounce curto e sem tráfego de rede. A busca remota ocorre apenas quando a pessoa envia o formulário ou quando outra rota publica uma solicitação explícita no `searchRouteRequest`.

O fluxo remoto é tipado de ponta a ponta:

```text
SearchRequest (Svelte) -> SearchOptions (RPC/Go) -> provider -> filtros locais -> NanoRank -> SearchPage
```

O método RPC `Search` aceita tanto o objeto tipado quanto a assinatura legada `query, limit, offset`. O offset legado é convertido em token numérico do provider público.

## Filtros

Os filtros combináveis incluem:

- texto livre, frase exata, termos obrigatórios e excluídos;
- vídeos, canais e playlists;
- uma ou mais janelas relativas de publicação, limite anterior, ordem e paginação;
- duração (incluindo mínimo/máximo), Shorts, evento ao vivo, legenda e características de vídeo;
- canal, idioma, região, categoria, tópico e localização;
- inscrições, progresso, favoritos, fila e rejeições locais.

Filtros pessoais são removidos do request antes de chamar o provider. Eles só são aplicados depois da resposta. Filtros exclusivos de vídeo não podem ser combinados com resultados de canal ou playlist; a validação falha de forma explícita.

Filtros estáveis do yt-dlp usam o provider público. Tipos não-vídeo e parâmetros específicos da Data API exigem uma conta conectada. Quando nenhum tipo é selecionado, a UI solicita vídeos, canais e playlists; num perfil visitante sem conta, somente essa busca inicial degrada explicitamente para vídeos públicos e mostra um aviso. Filtros oficiais escolhidos explicitamente continuam falhando fechados. O provider de produção usa o SDK oficial `google.golang.org/api/youtube/v3`, ligado sob demanda à sessão do perfil ativo.

O resultado inicial de `search.list` é enriquecido por `videos.list` para recuperar duração, categoria, visualizações e estado ao vivo e por `playlists.list` para a quantidade de itens. A ordem original é preservada. O valor **Qualquer** de Safe Search é enviado como `none`, pois omitir o parâmetro aplicaria `moderate` implicitamente e mudaria o significado escolhido na UI.

## NanoRank local

`NanoRank` é a ordenação padrão para relevância. Ele reordena apenas a página recebida e combina:

- posição original do provider, com peso principal;
- correspondência no título, canal e descrição;
- inscrição, favorito e fila locais;
- recência e popularidade com peso limitado.

O algoritmo é determinístico para os mesmos dados. Nenhum sinal local sai do dispositivo. Uma ordem explícita, como data ou título, desativa o NanoRank.

## Paginação

`SearchPage.next_page_token` é opaco para a UI. A Data API fornece seu token nativo. Tokens maiores que 2 KiB ou com caracteres de controle são rejeitados antes da rede. O provider yt-dlp usa offset decimal limitado a 500 resultados para impedir extrações sem limite em hardware modesto.

## Pesquisa global

`frontend/src/lib/stores/searchQueryStore.ts` define o contrato entre Header/rotas e `SearchView`:

- `submitGlobalSearch(query, filters)` publica uma intenção com `requestId` monotônico;
- `SearchView` consome cada `requestId` uma vez;
- apenas trocar para a aba Pesquisa não inicia rede;
- `remotePlaylistSelection` preserva título e contexto ao abrir uma playlist encontrada.

O Header e o roteador só dependem desse store; não importam detalhes de yt-dlp ou Google.

## Página de canal

Resultados de canal, nomes de canal nos cards, o gerenciamento de inscrições e os controles do player abrem `ChannelDetailView` por meio de `channelRouteStore`. A página possui abas Vídeos e Playlists, paginação, ações de fila/reprodução e inscrição local.

Vídeos públicos são listados pelo adaptador yt-dlp limitado e não são persistidos apenas por serem visualizados. Playlists usam a Data API oficial e, portanto, mostram erro recuperável quando não existe conta conectada. Abrir uma playlist preserva a seleção do canal; voltar do detalhe revela novamente a página de origem.

## Playlists remotas e locais

Uma playlist encontrada pode ser aberta sob demanda por `GetRemotePlaylistPage`, reproduzida, adicionada à fila, materializada no SQLite por `ImportRemotePlaylist` ou combinada com uma playlist local por `AddRemotePlaylistToPlaylist`.

A visualização remota é somente leitura e paginada. A importação:

- limita o total a 500 itens, usando 200 por padrão;
- persiste metadados antes de criar as referências locais;
- deduplica IDs remotos;
- remove a playlist local parcial se a inclusão falhar.

Isso preserva a separação do ADR-019: provider remoto e playlist local continuam sendo conceitos diferentes.

## Validação

```bash
go test ./internal/search ./internal/services ./internal/server
cd frontend && npm run check && npm run build
cd frontend && npx playwright test e2e/search.spec.ts e2e/playlists.spec.ts
```
