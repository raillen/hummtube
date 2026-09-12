---
id: security
status: canonical
---

# Segurança e Proteção de Dados — NanoTube Web

## 1. Princípios de Segurança
- **Zero Token Leakage**: Tokens OAuth, cookies e senhas nunca são gravados em logs ou no banco SQLite;
- **Cookies permanecem no navegador**: a preferência local guarda somente um seletor permitido (`navegador+keyring`). O backend rejeita perfis, caminhos e cofres arbitrários; o yt-dlp lê a sessão diretamente do navegador no momento da extração;
- **Armazenamento Seguro**: Refresh tokens usam o Keyring nativo do SO (`SecretStore`) e chaves separadas por `profile_id`;
- **Fallback explícito**: Se o Keyring estiver indisponível por condição reconhecida, o token fica somente na memória até o processo encerrar, a UI avisa o usuário e o bootstrap remove metadados transitórios. Permissão, integridade e I/O falham fechados;
- **Identidade estável**: Vínculos OAuth usam `provider_subject`; e-mail é apenas metadado mutável de apresentação;
- **Provisionamento fora da UI**: O aplicativo não recebe nem grava Client Secret. Credenciais OAuth do cliente são carregadas por variável de ambiente ou arquivo regular `0600`, sem symlink, provisionado externamente;
- **Isolamento por perfil**: Logout, exclusão e chaves de refresh token são sempre escopados ao perfil alvo. Guests não gravam token no Keyring e são limpos no bootstrap;
- **Commit OAuth consistente**: A identidade é validada antes de persistir, tentativas têm geração por perfil, commits são serializados e uma falha de metadados restaura o token anterior;
- **Superfície mínima**: `device_code` permanece no backend e configurações reservadas como `ui.active_profile` não podem ser escritas pelo RPC genérico;
- **Inventário sem valores**: o gerenciador visual recebe somente provedor, estado, armazenamento, datas disponíveis e capacidades de rotação/revogação. Client ID, Client Secret, refresh token, session key e identificadores internos do Keyring não atravessam o bridge;
- **Auditoria local delimitada**: eventos de conexão, rotação, revogação e falha de verificação são registrados por perfil com vocabulário fechado e detalhe limitado; material secreto é proibido nesse schema;
- **Content Security Policy (CSP)**: Wails v3 configurado com CSP restritivo, permitindo apenas execução de scripts do bundle local e conexões de mídia autorizadas;
- **Proteção de destinos remotos**: Invidious bloqueia hosts/IPs locais literais; IPTV exige HTTP(S), rejeita credenciais inline e bloqueia redirecionamento para outra origem. A validação de DNS/IP privado no momento da conexão ainda precisa ser concluída para fechar DNS rebinding em ambos os adaptadores;
- **Isolamento NanoSuite**: NanoTube, NanoIPTV e NanoMusic usam bancos e serviços de Keyring próprios. O bridge HTTP valida o par `service.method` e aplica uma allowlist por executável; o container Go compartilhado não é publicado diretamente como serviço Wails irrestrito;
- **Ponte RPC local**: Requisições com `Origin` externo são rejeitadas. Apenas a mesma origem e o servidor Vite local em `localhost/127.0.0.1:5173` são aceitos; não se usa CORS curinga.
- **Proxy de mídia desktop isolado**: uma porta efêmera em `127.0.0.1` evita as limitações de headers do `wails://`, mas atende somente tokens de mídia por `GET/HEAD/OPTIONS`; não há RPC ou assets nessa porta. O CORS `*` existe apenas nessa rota de recursos sem credenciais e cada token aleatório pertence ao plano publicado e expira no menor prazo entre seis horas, `PlaybackPlan.ExpiresAt` e a expiração conhecida das URLs. Substituir o plano ou chamar `MediaProxy.Revoke` revoga os tokens anteriores e cancela transferências em andamento. O contexto RPC cancela a publicação, não a reprodução já publicada; o mapa continua limitado a 128 recursos.

---

## Trust Boundaries & Disaster Recovery
- **Trust Boundaries**:
  - **Fronteira UI / WebKit**: Scripts do frontend rodam em sandbox estrita orquestrada pelo Wails v3, comunicando-se exclusivamente por IPC tipado com allowlist;
  - **Fronteira de Rede Externa**: Resoluções remotas HTTP possuem guardas ativas anti-SSRF; origens externas são sumariamente rejeitadas;
  - **Fronteira de Segredos**: Credenciais e tokens confidenciais residem exclusivamente no Keyring do SO (`SecretStore`) ou na memória volátil.
- **Disaster Recovery**:
  - Procedimento de recuperação automática contra corrupção do SQLite via restauração a partir do snapshot atômico mais recente cifrado com AES-256-GCM;
  - Fallback gracioso com purga de sessões corrompidas e retorno seguro ao estado padrão (*safe default*).
