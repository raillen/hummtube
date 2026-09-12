---
id: playback-backends
status: canonical
---

# Backends de Playback — NanoTube Web

## 1. Cascading Playback Resolver

O `CascadingResolver` orquestra múltiplos resolvedores com fallback inteligente:

```text
Entrada: PlaybackRequest(VideoID, SourceURL)
    │
    ├── 1. ExplicitYtDlpResolver (YouTube)
    │      Executa yt-dlp em modo JSON com PO Token / client android
    │      Retorna streams diretos combinados ou pares adaptativos
    │      (vídeo MP4/WebM + áudio compatível)
    │
    ├── 2. InvidiousResolver (Fallback YouTube)
    │      Consulta instâncias Invidious públicas/privadas com proteção SSRF
    │
    └── 3. DirectResolver (IPTV / Mídia Direta)
           Valida URL e formata PlaybackPlan com modo direto
```

A cadeia web começa sem cookies. Quando os provedores públicos terminam em
verificação anti-bot ou restrição e o perfil autorizou cookies, há uma única
tentativa autenticada com o cliente `web`. O cliente `android` não é usado
nesse ponto porque o próprio yt-dlp o ignora quando recebe cookies.

`PlaybackVariant.has_audio` informa se a resolução já contém áudio. Quando é `false`, o frontend reutiliza `PlaybackPlan.audio` ou a faixa `audio_only` compatível e mantém as duas mídias sincronizadas. O fallback Invidious segue o mesmo contrato.

O proxy publica o plano atomicamente e mantém variantes ainda não usadas disponíveis além dos cinco minutos iniciais. A validade comum é o menor prazo entre seis horas, `PlaybackPlan.ExpiresAt` e expirações conhecidas nas URLs; referências HLS herdam esse limite e podem encurtá-lo, nunca renová-lo. O contexto RPC só controla a publicação: seu encerramento não interrompe a reprodução. Outro plano publicado (inclusive direto) ou `MediaProxy.Revoke` invalida os tokens anteriores e cancela transferências. Planos inválidos/cancelados preservam o anterior; planos acima de 128 recursos são rejeitados sem publicação parcial. O mapa de recursos HLS permanece limitado pelo LRU existente.

Limitação: o bridge atual não possui notificação de stop ligada ao proxy; fechar apenas o player não revoga imediatamente o plano. Sem substituição/revogação explícita, vale o prazo absoluto acima, inclusive durante pausa. Expiração exige nova resolução; não há renovação automática de URL assinada. Esta correção não promove P-001 a decisão confirmada.

Os subprocessos do extrator têm timeout, saída limitada e encerramento da árvore de processos. Unix usa um grupo POSIX; Windows usa `taskkill /T` com fallback para `Process.Kill`. A separação por build tags evita dependências `syscall` específicas de Unix em builds Windows.
