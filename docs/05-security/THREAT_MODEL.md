---
id: threat-model
status: canonical
---

# Modelo de Ameaças (Threat Model) — NanoTube Web

## Vetores Analisados e Mitigações

1. **Ataques SSRF em Provedores Invidious / IPTV**:
   - *Risco*: Atacante força o backend a acessar endpoints de rede interna (`169.254.169.254`, `localhost`, `10.0.0.0/8`).
   - *Mitigação atual*: validação de esquema/host, bloqueio de host local literal no Invidious, rejeição de credenciais inline e redirecionamentos IPTV entre origens. Streams de playback resolvidos passam pelo proxy local opaco, que valida cada endereço retornado por DNS no `DialContext`, bloqueia redes não públicas e limita redirecionamentos.
   - *Lacuna rastreada*: os caminhos legados de consulta a Invidious/IPTV ainda precisam aplicar a mesma validação de todos os IPs obtidos por DNS e redirecionamentos; o proxy de playback já fecha essa fronteira no próprio `DialContext`.

2. **Injeção de SQL em Buscas Textuais**:
   - *Risco*: Metacaracteres em buscas (`%`, `_`, `\`) quebrando queries `LIKE` ou FTS5.
   - *Mitigação*: Sanitização com `escapeLikePattern` e escape de termos FTS5.

3. **Vazamento de Segredos em Backups**:
   - *Risco*: Snapshots exportados contendo refresh tokens ou credenciais de IPTV.
   - *Mitigação*: O exportador de backup ignora explicitamente tokens e credenciais, exigindo nova autenticação após restauração em outra máquina.

4. **Chamadas indevidas à ponte RPC local**:
   - *Risco*: Uma página externa tenta chamar operações mutáveis expostas em `127.0.0.1`.
   - *Mitigação*: o middleware rejeita `Origin` externo antes do dispatcher e permite apenas mesma origem ou o Vite local na porta 5173. O endpoint aceita `POST` e o frontend usa `application/json`.

5. **Contaminação entre contas ou perfis**:
   - *Risco*: um token global, troca de perfil durante OAuth ou logout sem escopo associa a conta e as ações pessoais ao usuário errado.
   - *Mitigação*: o fluxo fixa o `profile_id` de origem, usa geração monotônica e commit serializado por perfil. A identidade é validada antes da escrita; rollback preserva a sessão anterior. Tokens usam `google.refresh_token/<profile_id>` e metadados ficam em `profile_accounts`. Logout e exclusão operam somente no perfil alvo; `default` é protegido antes de tocar no SecretStore.

6. **Keyring ausente ou bloqueado**:
   - *Risco*: gravar refresh token em arquivo/SQLite como atalho ou exibir uma sessão volátil como persistente.
   - *Mitigação*: somente indisponibilidade reconhecida ativa fallback em memória, com aviso e `credential_state` no contrato/UI. O registro transitório é removido no próximo bootstrap; falha nessa limpeza põe APIs de conta/perfil em quarentena. Permissão, integridade e I/O interrompem o login.

7. **Resíduo de perfil guest após crash**:
   - *Risco*: um encerramento inesperado impede a exclusão normal de dados temporários.
   - *Mitigação*: guests são marcados no schema, nunca persistem refresh token e são removidos transacionalmente em todo bootstrap; referências com FK são apagadas por cascade e o perfil ativo volta para `default`.

8. **Exposição de hábitos pelo scrobble Last.fm**:
   - *Risco*: títulos, canais e horários de reprodução são enviados a um terceiro sem intenção clara, ou a session key vaza pelo RPC/SQLite.
   - *Mitigação*: integração desativada por padrão e proibida em guest; API key/shared secret vêm do ambiente, token temporário permanece em memória e session key fica somente no Keyring por perfil. O frontend envia scrobble ao terminar e o backend revalida o limiar. Desconectar remove o segredo antes de informar sucesso.

9. **Leitura arbitrária de perfis ao configurar cookies do yt-dlp**:
   - *Risco*: uma chamada RPC injeta caminho, perfil ou seletor de keyring para ampliar quais dados locais o subprocesso tenta ler.
   - *Mitigação*: o backend aceita somente navegadores e keyrings enumerados pela interface, sem caminhos ou perfis. O SQLite guarda apenas o seletor redigível; valores de cookies continuam no navegador e mensagens do extractor passam pela sanitização de segredos.

10. **Acesso local indevido ao proxy de mídia do desktop**:
   - *Risco*: outro processo ou página local tenta reutilizar a porta efêmera para acessar uma mídia resolvida.
   - *Mitigação*: o listener usa exclusivamente `127.0.0.1:0`, publica somente `GET/HEAD/OPTIONS` em `/api/media/` e não expõe RPC. Cada recurso exige token aleatório de 192 bits, expira no menor prazo entre seis horas e a expiração conhecida do plano/URLs e referencia somente destinos já aprovados pelo guard SSRF. A publicação de outro plano ou `MediaProxy.Revoke` invalida o anterior e cancela suas transferências; o armazenamento permanece limitado a 128 recursos. O CORS curinga é restrito a essa superfície de mídia para permitir o origin customizado do WebKit; URLs assinadas, tokens do proxy e headers privados não entram nos logs.
