# ADR-023: Inventário de credenciais somente por metadados

- Status: aceito
- Data: 2026-08-30
- Decisão relacionada: D-034

## Contexto

Contas Google e sessões Last.fm precisam de uma experiência visual para estado, rotação, revogação e auditoria. Expor valores, caminhos internos do Keyring ou permitir editar Client Secrets pela SPA ampliaria desnecessariamente a superfície de ataque.

## Decisão

O bridge expõe `Settings.GetSecretInventory`, que retorna apenas provedor, rótulo público, perfil, estado, classe de armazenamento, datas fornecidas pelo provedor, capacidades e avisos. O frontend nunca recebe Client ID, Client Secret, refresh token, session key nem a chave usada no Keyring.

Conexão, rotação, revogação e falha de verificação usam um vocabulário fechado em `credential_audit`, sempre escopado por `profile_id`. O detalhe é curto e não pode conter material secreto. Credenciais de cliente Google e Last.fm continuam provisionadas fora da UI; o inventário informa somente se a configuração está disponível.

## Alternativas consideradas

- Editor genérico de secrets na SPA: recusado porque levaria valores sensíveis ao WebView e ao bridge.
- Mostrar fragmentos mascarados: recusado porque ainda exige ler e transportar o valor secreto sem benefício operacional necessário.
- Auditoria em log de processo: recusado porque mistura eventos de segurança com logs, dificulta isolamento por perfil e aumenta o risco de vazamento acidental.

## Consequências

- Rotação de sessão Google reutiliza o fluxo OAuth; Last.fm reutiliza sua autorização dedicada.
- Expiração aparece como não informada quando o provedor não a publica; a UI não inventa prazos.
- A auditoria é local, não é telemetria e é removida por cascade com o perfil.
- Recuperação e provisionamento de credenciais de cliente permanecem tarefas externas documentadas.
