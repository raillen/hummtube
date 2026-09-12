---
id: nanosuite-plan
status: canonical
---

# NanoSuite — Limites dos Produtos

## Visão da Suíte

NanoSuite é uma família de aplicações multimídia ultra-leves e eficientes para Linux/Desktop que compartilham padrões de arquitetura, segurança e interface.

## Produtos atuais

1. **NanoTube**: cliente YouTube com catálogo local, inscrições, pesquisa, playlists, fila e ranking determinístico;
2. **NanoIPTV**: gestor e player dedicado de listas M3U/M3U+, VOD e guia EPG XMLTV;
3. **NanoMusic**: experiência dedicada a música e podcasts, com fila, playlists, modo somente áudio e Last.fm opcional.

Os três são executáveis independentes dentro deste monorepo transitório. Eles compartilham padrões e código genérico, mas não compartilham banco mutável, namespace de Keyring ou superfície RPC. Cada produto poderá ser movido futuramente para seu próprio repositório.

## Produto exploratório

**CalmTV Hub** permanece exploratório: uma interface 10-foot capaz de orquestrar aplicativos ou fontes sem voltar a fundir seus domínios.
