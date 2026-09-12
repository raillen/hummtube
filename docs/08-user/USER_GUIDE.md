---
id: user-guide
status: canonical
---

# Guia do Usuário — NanoTube Web

## 1. Primeiros Passos
- **Home**: Ao abrir o aplicativo, seu feed personalizado "Para Você" é exibido com os vídeos mais relevantes de suas inscrições e histórico. Use **Carregar mais** para revelar a próxima janela local sem perder o contexto.
- **Inscrições**: filtre vídeos sincronizados por hoje, ontem, 7/30 dias, conteúdo, categoria e estado assistido; use **Carregar mais** para manter os filtros e avançar;
- **Gerenciar canais**: selecione vários canais para favoritar, adicionar a pasta, editar tags ou desinscrever. Filtre por categoria, tags, período de inscrição e período do último vídeo visto; ordene por nome ou recência;
- **Atualizar**: Clique no botão com ícone de sincronização no topo para verificar novos vídeos nos seus canais;
- **Pesquisa**: Digite qualquer termo na barra de busca para ver resultados instantâneos do seu catálogo local ou pressione `Enter` para buscar no YouTube. Sem tipo marcado, a busca inclui tudo quando existe conta; no visitante, retorna vídeos públicos com aviso. Os filtros combináveis são aplicados em tempo real depois da primeira busca e são restaurados na próxima sessão;
- **Playlists & Regras**: Crie playlists manuais ou inteligentes através do menu lateral;
- **Modo TV**: Pressione `F10` ou clique no botão de TV no cabeçalho para ativar a interface otimizada para controle remoto/sofá.

## 2. Reprodução e fila

- **Retornar à interface**: no player expandido, use **Voltar à interface**. O vídeo continua tocando no miniplayer;
- **Miniplayer**: o mesmo vídeo permanece visível e reproduzindo com áudio e imagem. Use **Expandir** para voltar ao player completo;
- **Qualidade**: mova o ponteiro sobre o player ou foque os controles e abra o botão de monitor/qualidade. Use **Automática**, uma resolução disponível ou **Somente áudio**. A indicação “Vídeo e áudio adaptativos” identifica resoluções altas reproduzidas com faixas sincronizadas;
- **Vídeos restritos**: abra **Entrar → Cookies**, escolha o navegador que já possui acesso ao vídeo e, no Linux, o cofre usado por ele. Essa configuração é separada do OAuth da conta do NanoTube;
- **Fila**: use o botão de fila em cartões, playlists ou cabeçalho. O painel Sobre/Fila e a página **Fila** permitem arrastar, mover com botões, manter ou remover itens tocados, limpar somente tocados e salvar a sequência como playlist;
- **Reprodução sequencial**: ative **Reprodução automática** para tocar em ordem. A fila continua disponível depois de reiniciar e é separada por perfil;
- **Informações**: o botão de informações no player abre as abas **Sobre** (descrição completa) e **Fila**, além de atalhos para vídeos/playlists do canal;
- **Menu contextual**: clique com o botão direito em vídeos, canais, playlists e itens da fila para acessar ações rápidas. A IPTV fica fora deste fluxo por enquanto;
- **NanoMusic**: abra o aplicativo separado para pesquisar músicas/podcasts, organizar fila e playlists e usar **Somente áudio**;
- **Last.fm (Beta)**: no NanoMusic, o operador configura `NANOTUBE_LASTFM_API_KEY` e `NANOTUBE_LASTFM_SHARED_SECRET`; em **Ajustes**, use **Autorizar Last.fm** e **Concluir conexão**. A session key fica no Keyring `nanomusic`; o scrobble ocorre automaticamente em 50% ou quatro minutos, o que ocorrer primeiro;

## 3. Listas IPTV/M3U

- Abra o aplicativo **NanoIPTV** e use **Listas IPTV** ou o lápis de uma fonte para editar nome, URL, EPG, credenciais e modo de saída;
- Use **Diagnosticar** para verificar URL, DNS público e conexão TCP antes de salvar. O diagnóstico não baixa nem valida o conteúdo da M3U;
- Para provedores que entregam `.ts` com `output=mpegts`, prefira **HLS compatível**. A opção solicita `output=m3u8`; se o provedor não suportar esse parâmetro, use **Preservar a URL**;
- Domínios DNS alternativos não devem ser concatenados à lista. Mantenha o domínio fornecido pelo provedor para preservar o roteamento correto.

No primeiro uso, NanoIPTV e NanoMusic criam bancos próprios a partir dos dados legados aplicáveis: o NanoIPTV mantém apenas IPTV e preferências visuais, enquanto o NanoMusic remove todo o catálogo IPTV da cópia. O banco original do NanoTube não é alterado nem removido. Se o Keyring estiver indisponível durante a migração, reconecte a conta ou a credencial no aplicativo correspondente.

## 4. Perfis e conta Google

- Em **Configurações > Perfis locais**, crie ou alterne perfis persistentes. Cada perfil mantém seu próprio vínculo de conta e refresh token;
- Use **Adicionar conta** para criar e ativar um perfil próprio antes da autorização. O cabeçalho acompanha a troca sem reiniciar;
- Use **Usar como convidado** para uma sessão temporária. O guest não grava token no Keyring e seus vínculos são eliminados na próxima inicialização;
- Se o sistema não disponibilizar Keyring, o login continua somente durante o processo atual. A interface mostra **Sessão temporária**; será necessário autenticar novamente depois de fechar o aplicativo;
- O NanoTube não solicita Client Secret pela interface. Provisione o cliente Google com `NANOTUBE_GOOGLE_CLIENT_FILE` ou no arquivo externo `~/.config/nanotube-web/client_secret.json`; em Linux/macOS use um arquivo regular (não symlink) com `chmod 600`.
- O NanoMusic usa configuração própria em `~/.config/nanomusic/client_secret.json`, salvo quando `NANOTUBE_GOOGLE_CLIENT_FILE` aponta explicitamente para um cliente compartilhado.

## 5. Aparência, carregamento e bandeja

- Em **Ajustes > Conteúdo**, escolha densidade, qualidade de imagem, 12/24/36/48 vídeos por lote e oculte Shorts, lives, agendados, mixes ou conteúdo de membros;
- Em **Ajustes > Sistema**, ative/desative a bandeja, escolha perguntar, minimizar ou encerrar ao fechar e configure o salto de 5/10/15/30/60 segundos. A bandeja e o intervalo são atualizados sem reiniciar;
- Em **Ajustes > Segurança**, consulte estado, armazenamento, rotação/expiração disponível e auditoria local das credenciais. Valores secretos nunca são exibidos;
- O menu da bandeja controla reprodução, anterior/próximo, avanço/retrocesso, volume, mudo e abre a fila. Ele reutiliza o player ativo e não inicia uma segunda reprodução.
- A bandeja depende de um host StatusNotifier/AppIndicator fornecido pelo ambiente Linux. Em sessões sem esse host, o NanoTube informa a indisponibilidade e mantém o fechamento normal da janela, sem afetar a reprodução.
