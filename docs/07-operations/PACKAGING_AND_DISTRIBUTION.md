---
id: packaging-and-distribution
status: canonical
---

# Empacotamento e Distribuição Linux — NanoTube Web

Este documento registra o estado real e o plano dos formatos de distribuição para Linux/X11/Wayland. A presença de um arquivo em `packaging/` não significa que o formato esteja homologado.

## 1. Visão Geral dos Pacotes

| Formato | Distribuições alvo | Estado em 2026-09-07 | Arquivo | Comando |
|---|---|---|---|---|
| **.deb** | Ubuntu 24.04/Debian 13 amd64 | Preview gerada e validada | `packaging/deb/control` | `task package:deb` |
| **Arch pkg** | Arch Linux / Manjaro x86_64 | Pacote gerado via makepkg | `packaging/arch/PKGBUILD` | `task package:arch` |
| **AppImage** | Portável Linux x86_64 | AppDir montado e validado (AppImage requer `appimagetool`) | `packaging/appimage/AppRun` | `task package:appimage` |
| **.rpm** | Fedora/openSUSE/RHEL | Spec e task prontas; requer `rpmbuild` no host | `packaging/rpm/nanotube-web.spec` | `task package:rpm` |
| **Windows exe** | Windows 10/11 amd64 | Preview cross-compilada; modo servidor testado, WebView2 nativo pendente | `packaging/windows/build-windows.sh` | `task package:windows` |
| **Android APK** | Android TV / Mobile | Não suportado; arquitetura Wails v3 desktop não possui target APK nativo | — | — |

Os pacotes nativos só passam a `Suportado` após NT-011. AppImage ou Flatpak só podem ser promovidos após NT-012.

---

## 2. Estrutura de Instalação Padrão

Quando instalado nativamente no sistema de arquivos Linux, o NanoTube Web utiliza a hierarquia FHS (*Filesystem Hierarchy Standard*):

- **Executável Principal planejado**: `/usr/bin/nanotube-web`
- **Assets Estáticos do Frontend**: preferencialmente embutidos no binário; cópia externa só quando o empacotador provar necessidade
- **Lançador de Desktop**: `/usr/share/applications/nanotube-web.desktop`
- **Ícone do Sistema**: `/usr/share/icons/hicolor/scalable/apps/nanotube-web.svg`

---

## 3. Ações planejadas do Desktop Entry

O lançador `nanotube-web.desktop` expõe ações rápidas integradas ao ambiente de desktop (GNOME, KDE Plasma, XFCE):
- **Execução Normal**: Abre a interface web desktop no navegador/webview padrão.
- **Ação Modo TV**: Inicia diretamente no layout 10-foot UI com controle direcional ativado via `nanotube-web --tv`.

---

## 4. Dependências de Sistema

- **Obrigatórias**: `glibc >= 2.38`, `ca-certificates`, GTK 4 e WebKitGTK 6. A ABI desta preview foi compilada contra glibc 2.38; Debian 12 não é alvo deste artefato.
- **Recomendadas / Opcionais**:
  - `yt-dlp`: Extração avançada de streams YouTube e formatos adaptativos.
  - `deno` ou `nodejs >= 22`: Runtime JavaScript para resolução de PO Tokens e desafios EJS.
  - `yt-dlp-ejs`: pacote compatível com a mesma versão do yt-dlp. Sem ele, o NanoTube preserva o fallback 360p quando o YouTube exige solver.
  - plugins GStreamer (`base`, `good`, `bad`, `libav`): codecs usuais do WebView.

## 5. Preview reproduzível

```bash
task package:deb
task package:deb:verify
(cd build && sha256sum -c nanotube-web_0.1.0_amd64.deb.sha256)
```

O pacote contém somente `/usr/bin/nanotube-web`, o lançador e o ícone NanoTube; NanoIPTV/NanoMusic ficam fora. O host de desenvolvimento não possui `dpkg-deb`/`lintian`, portanto ainda falta validar instalação, upgrade, remoção e execução em uma imagem Debian/Ubuntu limpa.

## 6. Verificação de ferramentas

Antes de uma operação longa, é possível verificar as dependências do ambiente:

```bash
task tooling:check TARGET=verify
task tooling:check TARGET=deb
task tooling:check TARGET=appimage
task tooling:check TARGET=sbom
```

Ferramentas obrigatórias ausentes interrompem a operação com uma mensagem explícita. `dpkg-deb`, `lintian`, `appimagetool`, `rpmlint` e `namcap` são opcionais para validações adicionais; a ausência deles é reportada como aviso, nunca como uma homologação implícita.

## 7. AppImage

`task package:appimage` monta um AppDir somente do NanoTube, recompilando o bundle e o executável com os assets desse produto. NanoIPTV e NanoMusic não são copiados para o AppDir. Quando `appimagetool` está instalado, o comando também gera o arquivo `.AppImage` e seu `.sha256`; sem ele, o AppDir permanece disponível para empacotamento posterior.

Valide o AppDir ou um artefato já gerado com:

```bash
task package:appimage:verify
task package:appimage:verify APPIMAGE_TARGET=build/NanoTube-Web-0.1.0-x86_64.AppImage
```

O AppImage ainda é experimental: a validação estrutural não substitui smoke em hosts Linux com WebKitGTK compatível, Keyring, tray e aceleração gráfica.

## 8. SBOM e procedência

O projeto gera um inventário CycloneDX 1.5 das dependências de build (módulos Go e pacotes npm) sem incluir tokens, cookies ou valores de configuração:

```bash
task package:sbom
task package:sbom:verify
```

O arquivo é gravado em `build/sbom/nanotube-web-<versão>.cdx.json`. Ele é um inventário de dependências do build; ainda falta assinar o SBOM, produzir provenance atestável e associá-los a uma pipeline de release antes da promoção para distribuição pública.

O `npm ci` deste workspace também reporta um advisory moderado no Svelte 4 (a correção disponível é a migração para Svelte 5). Como a aplicação é SPA sem SSR, o risco de execução é reduzido, mas a migração permanece gate de segurança antes de uma release estável.

---

## Installation Paths & Ownership Lifecycle
- **Installation Paths**:
  - Binários de sistema: `/usr/bin/nanotube-web`, `/usr/bin/nanoiptv`, `/usr/bin/nanomusic`;
  - Dados locais e SQLite: `~/.local/share/nanotube-web/`;
  - Configurações do usuário: `~/.config/nanotube-web/`;
  - Cache de thumbnails e buffers: `~/.cache/nanotube-web/`.
- **Ownership & Permissões**: Arquivos de dados pertencem exclusivamente ao usuário executor (UID local, permissão `0600`/`0700`).

## Rollback Strategy & Uninstall Safety
- **Rollback Strategy**:
  - Em caso de falha de atualização, os gerenciadores nativos (APT, Pacman, DNF) revertem para a versão estável anterior;
  - As migrações de banco Goose contam com passos reversíveis garantindo integridade dos dados históricos.
- **Uninstall Safety**:
  - A remoção padrão de pacotes desinstala somente os executáveis de `/usr/bin` e lançadores desktop;
  - O banco SQLite, playlists e credenciais do Keyring são salvaguardados com segurança, salvo se purga total for requisitada.
