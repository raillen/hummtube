---
id: nanoiptv
status: canonical
---

# NanoIPTV — Aplicativo IPTV e EPG

## 1. Visão Geral
O NanoIPTV é um executável independente dedicado a fontes M3U/M3U8, VOD e guias XMLTV. Ele reutiliza o player e o núcleo IPTV do monorepo, mas não expõe serviços, navegação, fila ou dados do NanoTube.

Dados ficam em `$XDG_DATA_HOME/nanoiptv/nanoiptv.db` e credenciais no serviço de Keyring `nanoiptv`. No primeiro uso, se esse banco não existir e houver um banco legado do NanoTube, o launcher cria um snapshot, remove da cópia todos os dados YouTube/conta/fila e copia as referências IPTV conhecidas sem apagar a origem.

## 2. Segurança de Credenciais

- Credenciais de listas Xtream / M3U autenticadas são salvas com segurança no `domain.SecretStore` (Keyring nativo exclusivo do NanoIPTV);
- URLs com usuário e senha nunca são gravadas em texto plano no banco de dados SQLite.
- Ao editar uma fonte, os campos `username` e `password` são extraídos da URL (quando presentes), removidos da configuração persistida e gravados somente no keyring;
- A exclusão de uma fonte também remove a referência de credencial correspondente do keyring.

## 3. Edição, diagnóstico e formato de saída

- A tela **Fontes** permite editar nome, playlist, EPG, estado habilitado e credenciais sem recriar o catálogo;
- **Diagnosticar** valida o esquema HTTP(S), resolve DNS e testa conexão TCP. O resultado informa apenas host, porta e endereços públicos; caminho, query string e segredos são redigidos;
- O diagnóstico não autentica no provedor nem garante que o corpo seja uma M3U válida. A sincronização continua sendo a verificação definitiva do conteúdo;
- O modo **HLS compatível** troca `output=mpegts`/outro valor pelo parâmetro `output=m3u8`, quando o provedor aceita essa convenção. **Preservar a URL** mantém os parâmetros originais;
- Domínios DNS alternativos não são anexados nem substituídos automaticamente: o host informado é preservado, pois o provedor pode depender de roteamento por nome (virtual host).

## 4. Guia de Programação (EPG)

- Parse concorrente e indexação de programas XMLTV com visualização em linha do tempo na UI.

## 5. Importação e identidade do catálogo

- A sincronização processa a lista em lotes limitados e faz `UPSERT` por `iptv_items.id`; IDs repetidos na mesma M3U não anulam toda a importação.
- O ID derivado usa metadados estáveis do provedor e não inclui a URL efêmera do stream. Ao tocar, o backend relê a playlist e localiza a versão atual do item por essa identidade.
- O total retornado pela sincronização vem do catálogo persistido, inclusive no caminho de importação por streaming.
- Entradas que não puderem ser classificadas como TV, filme ou série permanecem acessíveis na aba **Outros**.

O fluxo suportado atualmente é M3U/M3U+ por HTTP(S), com XMLTV opcional. A interface não anuncia integração Xtream API independente; listas Xtream convertidas para M3U continuam compatíveis.
