---
id: troubleshooting
status: canonical
---

# Guia de Troubleshooting — NanoTube Web

## Problemas Comuns e Soluções

1. **Erro de HTTP 403 no Playback**:
   - *Causa*: YouTube bloqueou o cliente ou assinatura expirou.
   - *Solução*: Atualize o `yt-dlp` no sistema ou ative o fallback Invidious nas Configurações.

2. **Miniaturas não carregam**:
   - *Causa*: Falha de rede ou permissão na pasta `~/.cache/nanotube-web/`.
   - *Solução*: Verifique a conectividade ou limpe a pasta de cache nas Configurações.

3. **Falha de Autenticação OAuth**:
   - *Causa*: Credencial OAuth incompatível, porta do loopback bloqueada, Keyring indisponível ou autorização não concluída.
   - *Solução*: Confira a mensagem exibida pelo estado de login, valide o tipo da credencial no Google Cloud, verifique o Keyring e tente novamente. Para código de dispositivo, a credencial precisa permitir esse fluxo.

4. **A interface parece salvar, mas os dados somem**:
   - *Causa*: O frontend não alcançou `/api/rpc`; builds antigos podiam cair silenciosamente em dados simulados.
   - *Solução*: No desktop, inicie com `task dev`; o Wails serve `/api/rpc` no mesmo origin e gerencia o Vite automaticamente. A porta 8999 só é necessária no modo web/servidor independente. Mocks de UI exigem `VITE_ENABLE_MOCKS=true` explicitamente.

5. **`task dev` não inicia ou informa porta ocupada**:
   - *Causa*: A porta do Vite já está em uso, `wails3` não está no `PATH` ou as dependências frontend ainda não puderam ser instaladas.
   - *Solução*: Encerre o processo que ocupa 5173 ou execute `task dev VITE_PORT=9245`. Confirme `wails3 dev --help` e `npm --version` no mesmo terminal.

6. **Lista M3U sincroniza com zero itens ou erro de ID duplicado**:
   - *Causa*: O provedor repetiu `tvg-id` ou mudou URLs efêmeras dos streams.
   - *Solução*: Ressincronize com a versão atual. A importação usa UPSERT e identidade estável; itens sem categoria ficam em **Outros**.

7. **Vídeo resolve, mas o player web não inicia**:
   - *Causa*: Plano destinado ao `mpv-ytdl-hook`, stream expirado, faixa adaptativa sem áudio compatível ou formato não suportado pelo WebView.
   - *Solução*: Confira o diagnóstico do `yt-dlp`, atualize-o e tente novamente. O backend web aceita mídia combinada, HLS ou um par adaptativo diretamente reproduzível e mostra o erro do resolver/player na interface.

8. **O menu de qualidade mostra apenas 360p**
   - *Causa*: O formato progressivo do YouTube costuma parar em 360p. Resoluções maiores são adaptativas e podem ser omitidas por SABR ou exigir PO Token.
   - *Solução*: Abra **Configurações → Diagnóstico** e confira yt-dlp, runtime JS, PO Token e a configuração redigida de extração. Instale `yt-dlp-ejs` compatível ou, conscientemente, ative **Configurações → Sistema → Reprodução e qualidade → Permitir baixar o solver EJS**. Quando as URLs adaptativas são liberadas, o NanoTube lista e sincroniza 720p/1080p ou resoluções maiores automaticamente. O aviso no menu não afirma mais que o yt-dlp está desatualizado sem consultar o diagnóstico.

9. **Lista IPTV responde, mas os canais não tocam**:
   - *Causa*: A URL `output=mpegts` pode entregar streams `.ts` que o WebView não consegue abrir diretamente, ou o provedor pode exigir o host original para roteamento.
   - *Solução*: Execute **Diagnosticar**, escolha **HLS compatível** e sincronize novamente. Essa opção solicita `output=m3u8`; se o provedor não aceitar, use **Preservar a URL**. Não concatene DNS alternativos à URL: aliases podem resolver para o mesmo IP, mas não substituem o virtual host esperado pelo provedor.

10. **Vídeo restrito pede cookies mesmo após selecionar o navegador**:
   - *Causa*: o navegador está autenticado, mas o yt-dlp escolheu o cofre incorreto e não conseguiu descriptografar a sessão. Isso é comum em desktops leves que usam Secret Service sem se identificarem como GNOME.
   - *Solução*: abra **Entrar → Cookies**, selecione o navegador e o cofre correspondente. Use **GNOME Keyring / Secret Service** em GNOME e ambientes compatíveis; use **KWallet** no KDE. A tentativa autenticada usa o cliente web, pois o yt-dlp ignora cookies com o cliente Android. Feche e abra o NanoTube após alterar uma instalação externa do yt-dlp/provider.

11. **`ERR systray error: failed to register: The name is not activatable`**:
   - *Causa*: a sessão não tem um host StatusNotifierItem/AppIndicator, algo comum no GNOME sem a extensão AppIndicator ou em sessões mínimas. A bandeja é uma integração opcional do desktop, não uma dependência do player.
   - *Solução*: as versões atuais detectam o watcher D-Bus antes de criar o tray e seguem normalmente sem ele, mantendo o botão de fechar funcionando. Para habilitar o ícone, instale/ative um host SNI (por exemplo, AppIndicator no GNOME) e confirme com `busctl --user status org.kde.StatusNotifierWatcher`. Se o aviso continuar após a atualização, confirme que o binário novo foi instalado; uma sessão sem watcher continuará sem tray, por design.

12. **Avisos `Overriding existing handler for signal 10` ou `MESA-INTEL ... Vulkan support is incomplete`**:
   - *Causa*: o primeiro é um aviso do JavaScriptCore sobre o sinal usado pelo coletor de lixo; os demais são mensagens do driver Mesa para Ivy Bridge, cuja implementação Vulkan é parcial. Nenhum deles indica falha de autenticação, resolução ou reprodução por si só.
   - *Solução*: não defina `JSC_SIGNAL_FOR_GC` nem force renderização por software como configuração padrão. Se houver artefatos gráficos ou travamento, teste apenas para diagnóstico com `LIBGL_ALWAYS_SOFTWARE=1`; se o player continuar funcional, remova a variável e mantenha a aceleração normal.

13. **Não há terminal disponível para consultar a falha de reprodução**:
   - *Solução*: abra **Configurações → Sistema → Diagnósticos do Sistema**. O painel mostra e copia os últimos eventos sanitizados do processo. O arquivo completo fica em `~/.local/state/nanotube-web/nanotube-web.log`, é privado ao usuário e tem rotação limitada. No modo `task dev`, `F12` e `Ctrl+Shift+I` abrem o inspetor do WebView.

14. **O backend foi recompilado, mas a janela continua com o comportamento antigo**:
   - *Causa*: no Linux, um executável em uso pode aparecer como `(deleted)` depois que o watcher substitui o arquivo; o processo antigo continua em memória até ser reiniciado.
   - *Solução*: encerre a janela e execute novamente `task dev`. Confirme que o novo início aparece no painel de diagnóstico/log antes de repetir o teste.
