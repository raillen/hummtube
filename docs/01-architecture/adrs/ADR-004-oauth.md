---
id: adr-004-oauth
status: accepted
---

# ADR-004: Autenticação OAuth2 PKCE com Keyring do Sistema

## Contexto
O usuário precisa sincronizar suas inscrições do YouTube de forma segura sem expor senhas ou armazenar tokens em arquivos de texto plano.

## Decisão
Usar fluxo OAuth2 Authorization Code com PKCE via navegador do sistema e loopback local (`http://127.0.0.1:<port>/callback`).
O token de atualização (*refresh token*) é gravado no **Keyring nativo do sistema operacional** (DBus Secret Service no Linux / go-keyring), sob uma chave escopada ao perfil local. A identidade persistida usa `provider_subject`, não e-mail.

Quando o Keyring estiver indisponível por condição reconhecida, o fluxo pode manter o token somente em memória, deve avisar explicitamente o usuário e remover metadados transitórios no próximo bootstrap. Permissão, integridade e I/O falham fechados. O fallback não autoriza gravar o token em SQLite, backup, arquivo JSON ou log. Perfis guest sempre usam esse modo efêmero.

Tentativas concorrentes usam geração monotônica por perfil. A identidade é validada antes da persistência, o commit token+metadados é serializado e restaura a credencial anterior quando a gravação de metadados falha. O segredo `device_code` nunca faz parte do contrato público.

Credenciais OAuth do cliente são provisionadas fora da interface; o aplicativo pode ler configuração externa, mas não recebe nem escreve Client Secret.
