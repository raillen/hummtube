---
id: youtube-playback-modernization
status: canonical
---

# Modernização de Playback e PO Token — NanoTube Web

## 1. Desafios Antibot do YouTube
O YouTube utiliza desafios JavaScript complexos (Botguard / Attestation / PO Tokens) para bloquear ferramentas de extração e limitar vídeos a formatos de baixa qualidade.

## 2. Estratégia de Mitigação no NanoTube Web
1. **Cliente Primário**: `android` e `ios` (emulam clientes mobile nativos, que dispensam desafios web complexos na maioria dos casos);
2. **PO Token Provider**: Suporte a execução de scripts via Deno ou Node.js (`bgutil-ytdlp-pot-provider`) quando o cliente `mweb` é acionado;
3. **Muxing & Headers**: Headers HTTP estruturados (User-Agent, Referer, Cookies efêmeros) são passados diretamente na requisição de streaming.

## 3. Descoberta local do provider

O `LoadYtdlConfig` preserva a configuração explícita de `NANOTUBE_YTDL_POT_PROVIDER`. Quando ela não existe, procura uma instalação completa do provider em:

- `~/.local/share/nanotube/runtime/bgutil-ytdlp-pot-provider/server`;
- `~/bgutil-ytdlp-pot-provider/server`, caminho padrão do projeto upstream.

A descoberta exige `src/generate_once.ts` ou `build/generate_once.js` e o plugin do yt-dlp. O aplicativo não baixa nem executa dependências silenciosamente. Com provider válido, a cadeia web prioriza `mweb`, que pode devolver vídeo adaptativo até a resolução realmente oferecida pelo vídeo.

## 4. Cookies para conteúdo restrito

OAuth da YouTube Data API não autoriza a extração de mídia. Vídeos de membros ou com confirmação de idade exigem uma sessão do navegador que já tenha acesso. A UI persiste somente o seletor validado, como `brave+gnomekeyring`; os valores dos cookies permanecem no perfil do navegador e nunca entram no SQLite ou nos logs do NanoTube.

## 5. Runtime gerenciado e rollback

`internal/playback/media_proxy.go` mantém as URLs assinadas fora do WebView:
`ResolveMedia` registra cada faixa e devolve somente `/api/media/<token>`; o
servidor local encaminha Range/HEAD, filtra headers e rejeita destinos privados
com um discador que resolve DNS novamente. Manifests HLS são limitados e têm
playlists, segmentos e chaves HTTP reescritos para tokens locais. A sessão é
curta, limitada e não transporta cookies ou `Authorization`.

`internal/playback/runtime_store.go` fornece a fundação local para componentes
aprovados: cada manifesto é validado, todo arquivo declarado precisa de hash
SHA-256, os arquivos são copiados para uma pasta versionada e a publicação ocorre
por `rename` atômico. O arquivo `active.json` mantém a versão atual e a anterior;
`Activate` e `Rollback` trocam somente esse ponteiro. Caminhos absolutos,
traversal, symlinks, escapes da origem, alterações posteriores e hashes
incorretos são rejeitados antes do uso. Quando existe uma versão ativa, o
resolver prioriza o `yt-dlp` e o runtime JS desse diretório; sem ela, mantém o
fallback para instalações do sistema/PATH.

Essa camada não baixa código nem implementa ainda um canal remoto ou botão de
atualização na UI. A distribuição do runtime continua exigindo pacote assinado,
SBOM/provenance e consentimento explícito antes de qualquer download.
