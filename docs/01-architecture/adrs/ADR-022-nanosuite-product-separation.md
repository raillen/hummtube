# ADR-022: Separação executável de NanoTube, NanoIPTV e NanoMusic

- Status: aceito
- Data: 2026-08-30
- Decisão relacionada: D-033

## Contexto

IPTV e música cresceram como workspaces dentro do NanoTube, mas possuem públicos, segurança, navegação e ciclos de evolução diferentes. Manter tudo em uma única superfície aumentava o escopo do cliente YouTube e tornava a futura criação de repositórios próprios mais arriscada.

## Decisão

O repositório funciona temporariamente como monorepo de três produtos:

- `cmd/nanotube-web` + bundle `frontend/dist/nanotube`;
- `cmd/nanoiptv` + bundle `frontend/dist/nanoiptv`;
- `cmd/nanomusic` + bundle `frontend/dist/nanomusic`.

Cada executável possui `product.Spec`, allowlist RPC por serviço/método, diretório XDG, arquivo SQLite e serviço de Keyring exclusivos. O handler valida o par `service.method` antes do dispatch. O serviço Wails genérico não é publicado diretamente; a superfície suportada passa pelo handler RPC restrito do produto.

No primeiro uso de NanoIPTV ou NanoMusic, se o banco próprio ainda não existe, um snapshot SQLite não destrutivo do banco legado preserva os dados atuais. Logo após migrar o schema, linhas fora do domínio do produto são removidas da cópia: NanoIPTV mantém apenas IPTV e preferências visuais; NanoMusic remove todo o catálogo IPTV. Credenciais conhecidas são copiadas entre namespaces de Keyring quando o cofre está disponível. A origem nunca é alterada.

## Alternativas consideradas

- Apenas esconder as abas: recusado porque manteria contratos, dados e credenciais compartilhados.
- Criar imediatamente três repositórios: adiado para evitar três migrações simultâneas sem antes estabilizar os boundaries.
- Compartilhar um único banco entre executáveis: recusado por acoplamento de schema, concorrência e risco de extração futura.

## Consequências

- Os produtos podem ser executados, versionados e testados separadamente desde já.
- Código genérico de player, storage e tipos ainda é compartilhado no monorepo; compartilhar implementação não autoriza compartilhar estado ou RPC.
- O snapshot inicial mantém o schema legado por compatibilidade de código, mas poda dados de outros domínios. Migrações fisicamente menores ficam para a extração dos repositórios.
- Empacotamento Linux específico de NanoIPTV/NanoMusic ainda deve ser criado antes de releases distribuíveis.
