---
id: youtube-and-auth
status: canonical
---

# YouTube e Autenticação OAuth2 — NanoTube Web

## 1. Fluxo de Autorização
1. O usuário aciona "Conectar Conta do YouTube" em Configurações;
2. O backend Go inicia um listener HTTP efêmero em `127.0.0.1:<random-port>/callback`;
3. Abre o navegador padrão do sistema apontando para a URL de autorização do Google com PKCE (`code_challenge`);
4. O Google redireciona de volta para o loopback local com o `code`;
5. O backend troca o código pelo `access_token` e `refresh_token`;
6. O backend consulta o `userinfo`, exige o identificador estável do provedor (`provider_subject`) e vincula a conta ao perfil que iniciou o fluxo;
7. O `refresh_token` é gravado no Keyring do SO sob uma chave derivada do `profile_id`;
8. O loopback responde uma página de sucesso amigável e encerra o servidor local.

O pedido de autorização força consentimento para que uma reconexão também possa devolver `refresh_token`. O frontend acompanha a conclusão assíncrona por `GetLoginStatus`, exibe a falha real do backend e aplica um tempo limite; não interpreta ausência de resposta como login concluído.

## 2. Fluxo por código de dispositivo

O backend solicita o código, faz polling com o intervalo e validade retornados pelo Google e publica os mesmos estados `pending`, `connected` ou `error`. Esse fluxo depende de credenciais OAuth habilitadas pelo Google para device authorization; uma credencial incompatível é reportada na interface. No frontend há um único polling cancelável por tentativa (cancelado ao trocar de fluxo, fechar o modal ou desmontar), e o Modo TV abre o modal diretamente na aba de código do dispositivo.

## 3. Perfis e isolamento de conta

O perfil `default` é determinístico e recebe, sem perda, os metadados e o refresh token das versões anteriores. Cada perfil possui no máximo um vínculo em `profile_accounts`, identificado por `provider + provider_subject`; o e-mail é apenas metadado de exibição e nunca é usado como identidade autoritativa.

O perfil que inicia OAuth fica fixado ao fluxo assíncrono. Cada nova tentativa recebe uma geração monotônica por perfil: uma conclusão antiga não pode sobrescrever token ou identidade de uma tentativa mais nova. A identidade é validada com o access token ainda em memória; somente então o backend entra na seção de commit serializada, preserva a credencial anterior para rollback, grava o refresh token e atualiza os metadados. Trocar o perfil ativo enquanto o navegador ou o device code aguardam autorização não transfere a conta resultante para outro perfil.

O `device_code` usado no polling nunca atravessa o RPC. A interface recebe apenas `user_code`, URL de verificação e validade. Logout e exclusão removem apenas o token e os metadados do perfil alvo; se o SecretStore falhar, os metadados permanecem para que a remoção possa ser repetida. O perfil `default` é rejeitado antes de qualquer efeito destrutivo.

Perfis `guest` usam identificadores próprios, não escrevem tokens no Keyring e são removidos no próximo bootstrap, inclusive depois de encerramento inesperado. As FKs com `ON DELETE CASCADE` removem vínculos locais associados ao guest. Esta fundação isola identidade, sessão, credenciais e referências em `profile_members`; cada repositório de mídia deve consumir `ActiveProfileID(ctx)` ao aderir ao escopo multi-perfil.

## 4. Persistência da sessão e fallback

Somente metadados não sensíveis (`provider`, `provider_subject`, `email`, `connected_at` e tipo de persistência) ficam no SQLite. Refresh tokens usam a chave `google.refresh_token/<profile_id>` no `SecretStore`.

Quando o Keyring está indisponível por uma condição reconhecida, o login continua com o refresh token apenas na memória da sessão. O contrato devolve `session_persistence: "memory"`, `credential_state: "memory"` e um aviso explícito; os metadados transitórios são apagados no bootstrap seguinte para não apresentar uma conta como conectada sem token. Erros de permissão, integridade ou I/O não viram fallback silencioso.

Contas persistidas são verificadas ao serem lidas e expõem `credential_state` como `available`, `memory`, `unavailable` ou `missing`. Falha na limpeza de guests ou sessões transitórias coloca as APIs de perfil/conta em quarentena até um bootstrap conseguir concluir a limpeza.

O `TokenSource` OAuth é reutilizado dentro da sessão de cada perfil para evitar uma troca de refresh token a cada chamada da Data API. Ele é invalidado ao armazenar uma nova credencial ou limpar a sessão. A criação do cliente oficial captura o perfil ativo no começo da operação, rejeita guest/conta sem credencial e usa timeout de 30 segundos; trocar de perfil durante a requisição não troca a identidade já capturada.

## 5. Provider oficial do catálogo

`internal/youtubeapi.Provider` implementa o port `domain.YouTubeProvider` com o SDK oficial Google. Ele cobre pesquisa tipada, inscrições, metadados de canais e uploads sem devolver tipos do SDK ao domínio ou ao frontend. Respostas de erro são traduzidas por status sem incluir corpo remoto ou cabeçalhos, reduzindo o risco de vazar detalhes de autenticação.

## 6. Provisionamento de credenciais OAuth do aplicativo

O NanoTube carrega a configuração do cliente Google de `NANOTUBE_GOOGLE_CLIENT_FILE` ou do arquivo externo `~/.config/nanotube-web/client_secret.json`. A interface não recebe Client Secret e o aplicativo não oferece operação para gravá-lo em JSON. Em sistemas POSIX o arquivo precisa ser regular, não pode ser symlink e deve usar permissão `0600` (sem bits de grupo/outros).
