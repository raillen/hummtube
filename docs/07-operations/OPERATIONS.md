---
id: operations
status: canonical
---

# Operações e Releases — NanoTube Web

## 1. Processo de Release
1. Executar gate completo de validação: `task verify`;
2. Incrementar versão no `Taskfile.yml` e `internal/diagnostics/version.go`;
3. Gerar artefatos de produção: `task build`;
4. Validar smoke test manual e empacotamento Linux (Debian, Arch, RPM, Tarball).

## 2. Matriz de smoke test da bandeja Linux

Antes de publicar, execute em pelo menos uma sessão de cada coluna disponível:

| Ambiente com host SNI/AppIndicator | Exibir/ocultar sem reiniciar | Menu atualiza salto | Controles do player | Fechar sem tray encerra |
|---|---:|---:|---:|---:|
| X11 + desktop com AppIndicator/SNI | obrigatório | obrigatório | obrigatório | obrigatório |
| Wayland + GNOME com extensão AppIndicator | obrigatório | obrigatório | obrigatório | obrigatório |
| Wayland + KDE Plasma | obrigatório | obrigatório | obrigatório | obrigatório |
| WM leve X11 (IceWM/Xfce/LXQt) com host SNI | obrigatório | obrigatório | obrigatório | obrigatório |
| Qualquer sessão sem host SNI | não aplicável | não aplicável | não aplicável | obrigatório |

O teste automatizado cobre validação/persistência do toggle, serialização das mudanças e reconstrução dinâmica do menu. No Linux, o runtime emite `org.kde.StatusNotifierItem.NewStatus` com `Active` ou `Passive`, contornando o `Show/Hide` ainda vazio no backend Linux do Wails v3 beta.13.

Antes do smoke visual, confirme a presença do host:

```bash
busctl --user status org.kde.StatusNotifierWatcher
```

Sem esse serviço, o NanoTube não registra o tray e segue com o fechamento normal da janela. Nenhum ícone SNI será apresentado até um host ser instalado/ativado. A presença e o comportamento visual permanecem um smoke nativo, não um teste de DOM.

---

## Command Surface & CLI Reference
- **Comandos de Automação do Projeto via Taskfile**:
  - `task dev`: Inicia o ambiente de desenvolvimento Wails v3 com Hot Reload;
  - `task test`: Executa a bateria de testes unitários com detector de corrida (`go test -race ./...`);
  - `task build`: Compila os binários executáveis autocontidos (`nanotube-web`, `nanoiptv`, `nanomusic`);
  - `task verify`: Executa o quality gate completo de conformidade, compilação e testes.

## Exit Codes & Execution Examples
- **Exit Codes**:
  - `0`: Execução concluída com sucesso (todas as validações aprovadas);
  - `1`: Falha de asserção em testes, erro de compilação ou violação de contrato;
  - `2`: Erro de uso, flags inválidas ou dependência de tooling ausente.
- **Execution Examples**:
  ```bash
  # Executar gate padrão de verificação
  task verify

  # Iniciar desenvolvimento do NanoTube
  task dev
  ```
