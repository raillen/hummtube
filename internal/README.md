# Núcleo Interno da Aplicação (`internal/`)

## O que é este diretório?
Contém todo o código-fonte proprietário de backend do projeto, protegido contra importações externas pelo compilador Go.

## Para que serve?
Implementa os princípios de Clean Architecture: o domínio central (`internal/domain`) permanece completamente agnóstico de frameworks, enquanto adaptadores de infraestrutura e serviços orquestram os casos de uso.

## Inventário
- `domain/`: Entidades e modelos de domínio puros;
- `services/`: Serviços de aplicação e orquestração de casos de uso;
- `storage/`: Camada de persistência SQLite, queries e migrações Goose;
- `playback/`: Motor de resolução e cascading de streams (`PlaybackResolver`);
- `search/`: Mecanismo híbrido de busca (FTS5 local + Data API remota);
- `auth/`: Autenticação OAuth2 PKCE e integração com o Keyring do SO;
- `sync/`: Sincronização incremental de inscrições e feeds;
- `backup/`: Exportação e importação atômica de snapshots cifrados (AES-256-GCM);
- `product/`: Especificação de produtos da NanoSuite e allowlists RPC;
- `launcher/`: Orquestrador de janelas desktop e ciclo de vida Wails v3;
- `server/`: Servidor bridge HTTP/RPC local;
- `desktop/`: Integrações com o ambiente desktop Linux (System Tray, SNI, atalhos);
- `diagnostics/`: Telemetria técnica, logs e métricas de execução;
- `iptv/`: Motor de parsing e reprodução de IPTV (NanoIPTV);
- `lastfm/`: Integração e scrobbling de faixas musicais (NanoMusic);
- `suggestions/`: Algoritmos de sugestões e autocompletion;
- `telemetry/`: Métricas locais não invasivas de uso de hardware;
- `youtubeapi/`: Cliente oficial da YouTube Data API v3;
- `app/`: Container de injeção de dependências do aplicativo.
