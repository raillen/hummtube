---
id: build-and-tooling
status: canonical
---

# Build e Ferramentas — NanoTube Web

## Comandos Principais (Taskfile)

- `task dev`: Inicia NanoTube com Wails v3/HMR;
- `task dev:nanoiptv`: Inicia NanoIPTV com Wails v3/HMR;
- `task dev:nanomusic`: Inicia NanoMusic com Wails v3/HMR;
- `task test`: Executa testes do Go e do Frontend;
- `task lint`: Executa linters e verificação de tipos (`svelte-check`, `go vet`);
- `task tooling:check TARGET=verify`: Verifica ferramentas obrigatórias do ambiente;
- `task verify`: Executa o gate completo obrigatório antes de commits e merges;
- `task build`: Compila os três bundles e executáveis de produção.

### Release e distribuição

- `task package:deb`: Gera e valida a preview Debian amd64 do NanoTube;
- `task package:appimage`: Monta o AppDir NanoTube e gera AppImage quando `appimagetool` existe;
- `task package:appimage:verify`: Valida o AppDir ou um `.AppImage` sem abrir a aplicação;
- `task package:sbom`: Gera o SBOM CycloneDX das dependências Go/npm;
- `task package:sbom:verify`: Valida o SBOM produzido;
- `task package:rpm`: Gera o RPM NanoTube (requer `rpmbuild` no host);
- `task package:arch`: Gera o pacote Arch x86_64 via `makepkg`;
- `task package:windows`: Cross-compila o `.exe` amd64 em modo preview (janela WebView2 nativa não testada);

RPM e Arch possuem tasks e receitas funcionais; a homologação em imagens limpas Fedora/Arch permanece pendente. Android APK não é suportado: o runtime Wails v3 desktop não possui target APK nativo.

## Modo de desenvolvimento nativo

Cada task dev escolhe seu `build/config*.yml` e define `NANOSUITE_APP` para o Vite. O watcher nativo controla todo o ciclo:

1. instala as dependências npm quando necessário;
2. inicia o Vite em segundo plano com HMR;
3. compila o backend Go com símbolos de depuração;
4. abre a janela desktop;
5. recompila e reinicia o processo ao detectar alterações em arquivos Go.

O Wails fornece `WAILS_VITE_PORT` ao Vite e `FRONTEND_DEVSERVER_URL` ao runtime. O `AssetFileServerFS` encaminha os assets ao Vite durante o desenvolvimento, enquanto `/api/*` continua sendo atendido no mesmo origin interno da aplicação. Por isso, o modo desktop não abre uma porta RPC 8999 adicional.

O frontend usa npm e `package-lock.json` como fonte reproduzível de dependências. A task de instalação executa `npm ci`; não misture `pnpm` e npm no mesmo `node_modules`.

`npm run build` constrói os três targets em subdiretórios independentes. No modo servidor, cada executável resolve apenas seu `frontend/dist/<produto>`; os assets embutidos são fallback.

Para escolher outra porta:

```bash
task dev VITE_PORT=9245
task dev:nanoiptv VITE_PORT=9245
task dev:nanomusic VITE_PORT=9245
```

O script `./run-dev.sh` permanece apenas como wrapper de compatibilidade para o mesmo fluxo nativo.
