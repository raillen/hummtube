---
id: test-strategy
status: canonical
---

# Estratégia de Testes — NanoTube Web

## 1. Pirâmide de Testes

1. **Testes Unitários Go**: Cobrem domínio puro, parsing de RSS/M3U/XMLTV, criptografia de snapshots, regras de playlists inteligentes e algoritmo MMR (`internal/...`);
2. **Testes de Repositório SQLite**: Executam migrações reais e queries FTS5 em banco em memória (`:memory:`);
3. **Testes Unitários de Frontend**: Vitest testando componentes Svelte, stores reativas e cálculo de formatação;
4. **Testes de Integração & Mock**: Simulam chamadas da YouTube Data API e extratores yt-dlp usando fixtures de teste.

## 2. Bandeja do sistema

O controlador do tray possui testes com detector de corrida para toggle, propagação de falhas e reconstrução dinâmica do menu. No Linux, testes focados verificam o mapeamento do setting para os estados SNI `Active` e `Passive`.

O smoke nativo complementar deve observar o sinal `org.kde.StatusNotifierItem.NewStatus` e confirmar o resultado visual em um desktop que ofereça `org.kde.StatusNotifierWatcher`. Uma sessão sem watcher valida a emissão D-Bus, mas não conta como validação visual do ícone.

O boundary de subprocessos é testado com cancelamento e overflow no host Unix. A compilação de teste do pacote desktop com `GOOS=windows CGO_ENABLED=0 go test -c` protege a separação das implementações específicas de processo e tray.

---

## Quality Gates & Evidence Expectations
- **Unit & Concurrency Quality Gates**: `go test -v -race ./...` com 100% de aprovação e zero condições de corrida detectadas;
- **Frontend Quality Gates**: `npm run check` (`svelte-check`) e `npm test` executados sem erros;
- **Conformance & Formatting**: `gofmt -d .` e `govet ./...` sem advertências;
- **Evidence Expectations**: Cada modificação estrutural de domínio, player ou persistência requer evidências executáveis vinculadas antes da conclusão.
