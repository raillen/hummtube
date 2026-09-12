#!/usr/bin/env bash
set -euo pipefail

readonly ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
readonly PACKAGE_VERSION="${NANOTUBE_PACKAGE_VERSION:-0.1.0}"
readonly OUTPUT_DIR="${NANOTUBE_WINDOWS_OUTPUT_DIR:-${ROOT_DIR}/build/windows}"

require_command() {
    command -v "$1" >/dev/null 2>&1 || {
        printf 'erro: comando obrigatório não encontrado: %s\n' "$1" >&2
        exit 1
    }
}

main() {
    require_command go
    require_command npm

    [[ "$(uname -m)" == x86_64 ]] || {
        printf 'erro: cross-compile Windows gera somente amd64; host atual: %s\n' "$(uname -m)" >&2
        exit 1
    }

    printf '==> Compilando frontend NanoTube...\n'
    (cd "${ROOT_DIR}/frontend" && npm run build:nanotube)

    printf '==> Cross-compilando executável Windows amd64 (CGO_ENABLED=0)...\n'
    mkdir -p "${OUTPUT_DIR}"
    (
        cd "${ROOT_DIR}"
        CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build \
            -buildvcs=false \
            -trimpath \
            -ldflags "-s -w -X github.com/nanotube/nanotube-web/internal/diagnostics.Version=${PACKAGE_VERSION}" \
            -o "${OUTPUT_DIR}/nanotube-web.exe" \
            ./cmd/nanotube-web
    )
    (cd "${OUTPUT_DIR}" && sha256sum nanotube-web.exe >nanotube-web.exe.sha256)

    printf '==> Executável Windows gerado em: %s\n' "${OUTPUT_DIR}"
    printf '==> LIMITAÇÃO: janela Wails nativa (WebView2) NÃO testada; modo servidor HTTP é o fallback esperado.\n'
}

main "$@"
