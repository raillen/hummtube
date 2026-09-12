---
id: diagnostics-spec
status: canonical
---

# Especificação de Diagnósticos — NanoTube Web

## 1. Métricas Coletadas

- **Versão do Aplicativo**: Commit, Tag e data de build;
- **Sistema Operacional**: Kernel Linux, distribuição, servidor gráfico (Wayland/X11);
- **Extrator yt-dlp**: Caminho, versão detectada e status de desatualização;
- **Runtimes JavaScript**: Detecção de Deno, Node.js e QuickJS disponíveis para PO Token;
- **Aceleração Gráfica / Codecs**: Status de suporte VA-API / GPU no WebView;
- **Estado do Banco de Dados**: Versão atual de migração Goose, contagem de canais, vídeos e tamanho do arquivo SQLite em disco.
- **Métricas de execução**: contadores de resoluções de playback, hits do cache,
  falhas e latência máxima; contadores equivalentes de pesquisa. Nenhuma
  métrica armazena URL, query, vídeo, provider ou identificador de perfil.
- **Log local recente**: o painel de diagnóstico mostra as últimas 80 linhas do
  log persistente do produto. Queries de URL e pares que aparentem token,
  senha, cookie, autorização ou chave de API são redigidos antes de atravessar
  o bridge. O arquivo usa modo `0600`, gira em 2 MiB e mantém uma geração.

## 2. Widget de Uso de Recursos (Sidebar)

O cartão substitui o texto estático do rodapé da sidebar e exibe métricas leves
obtidas via `Settings.GetResourceUsage`:

| Métrica | Escopo | Fonte | Unidade |
|---|---|---|---|
| RAM (RSS) | Processo Go (sem WebView) | `/proc/self/status` → `VmRSS` | bytes (IEC: KiB/MiB/GiB) |
| CPU | Sistema (todos os cores) | `/proc/stat` → linha `cpu` | % (delta entre amostras) |
| Rede (↓/↑) | Sistema (sem loopback) | `/proc/net/dev` | bytes/s (IEC: KiB/s, MiB/s) |

A rede inclui tráfego local e interfaces virtuais; não mede exclusivamente
internet nem confirma conectividade. CPU exclui idle/iowait e não soma guest
novamente. A primeira amostra não tem taxas; falhas de leitura, reinício de
contadores ou troca de interfaces deixam a métrica indisponível até nova amostra.
Não há subprocessos, sondagens de rede, persistência ou coleta do relatório completo.

### Características

- **Plataforma**: Linux apenas. Em outros sistemas, todas as métricas retornam `null`
  e o cartão exibe "Recursos indisponíveis".
- **Throttle**: O backend (`ResourceSampler`) limita releituras a no máximo uma a
  cada 3 s. O frontend faz polling via `setTimeout` a cada 5 s, sem sobreposição
  de chamadas.
- **Pausa**: O polling é suspenso quando a sidebar está recolhida, a janela não
  está visível (`document.hidden`) ou a viewport é menor que `md` (768 px).
- **Cleanup**: O timer é limpo no `onDestroy` do componente via flag `disposed`.
- **Segurança**: Nenhuma informação de identificação, URL ou segredo é exposta.
  A rota está na allowlist RPC de `Settings`.
