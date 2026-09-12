#!/usr/bin/env bash
set -euo pipefail

readonly ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
readonly APPDIR="${ROOT_DIR}/build/AppDir"
readonly VERSION="${NANOTUBE_PACKAGE_VERSION:-0.1.0}"
readonly SOURCE_EPOCH="${SOURCE_DATE_EPOCH:-946684800}"
temporary_dir=""

cleanup() {
    if [[ -n "${temporary_dir}" ]]; then
        rm -rf -- "${temporary_dir}"
    fi
}

require_command() {
    command -v "$1" >/dev/null 2>&1 || {
        printf 'erro: comando obrigatório não encontrado: %s\n' "$1" >&2
        exit 1
    }
}

validate_inputs() {
    case "$(uname -m)" in
        x86_64|amd64) ;;
        *)
            printf 'erro: esta preview gera somente x86-64; host atual: %s\n' "$(uname -m)" >&2
            exit 1
            ;;
    esac
    [[ "${VERSION}" =~ ^[0-9][0-9A-Za-z.+:~_-]*$ ]] || {
        printf 'erro: versão inválida: %s\n' "${VERSION}" >&2
        exit 1
    }
    [[ "${SOURCE_EPOCH}" =~ ^[0-9]+$ ]] || {
        printf 'erro: SOURCE_DATE_EPOCH deve ser um inteiro não negativo\n' >&2
        exit 1
    }

    require_command cp
    require_command find
    require_command go
    require_command grep
    require_command install
    require_command mktemp
    require_command npm
    require_command readelf
    require_command rm
    require_command sed
    require_command sha256sum
    require_command file
}

build_frontend() {
    local nanotube_dist="${ROOT_DIR}/frontend/dist/nanotube"

    printf '==> Compilando somente o frontend NanoTube...\n'
    if [[ -L "${nanotube_dist}" ]]; then
        printf 'erro: o destino do frontend não pode ser um link simbólico\n' >&2
        exit 1
    fi
    mkdir -p "${nanotube_dist}"
    find "${nanotube_dist}" -mindepth 1 ! -name '.gitkeep' -delete
    (
        cd "${ROOT_DIR}/frontend"
        npm run build:nanotube
    )
    [[ -f "${nanotube_dist}/index.html" ]] || {
        printf 'erro: frontend NanoTube não foi gerado\n' >&2
        exit 1
    }
}

build_binary() {
    local binary_path="$1"
    local overlay_source="${temporary_dir}/assets.nanotube.go"
    local overlay_config="${temporary_dir}/overlay.json"

    printf '%s\n' \
        '// Package nanotube provê os assets estáticos embutidos do frontend SPA.' \
        'package nanotube' \
        '' \
        'import "embed"' \
        '' \
        '//go:embed all:frontend/dist/nanotube' \
        'var FrontendAssets embed.FS' >"${overlay_source}"
    printf '{"Replace":{"%s/assets.go":"%s"}}\n' \
        "${ROOT_DIR}" "${overlay_source}" >"${overlay_config}"

    printf '==> Compilando o executável NanoTube x86-64...\n'
    (
        cd "${ROOT_DIR}"
        CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go build \
            -buildvcs=false \
            -trimpath \
            -overlay "${overlay_config}" \
            -ldflags "-s -w -buildid= -X github.com/nanotube/nanotube-web/internal/diagnostics.Version=${VERSION}" \
            -o "${binary_path}" \
            ./cmd/nanotube-web
    )
}

stage_appdir() {
    local binary_path="$1"
    local nanotube_dist="${ROOT_DIR}/frontend/dist/nanotube"
    local appdir_dist="${APPDIR}/usr/share/nanotube-web/frontend/dist/nanotube"

    printf '==> Montando AppDir somente com NanoTube...\n'
    rm -rf -- "${APPDIR}"
    mkdir -p \
        "${APPDIR}/usr/bin" \
        "${appdir_dist}" \
        "${APPDIR}/usr/share/applications" \
        "${APPDIR}/usr/share/icons/hicolor/scalable/apps"

    install -m 0755 "${binary_path}" "${APPDIR}/usr/bin/nanotube-web"
    cp -a "${nanotube_dist}/." "${appdir_dist}/"
    install -m 0644 "${ROOT_DIR}/packaging/nanotube-web.desktop" "${APPDIR}/nanotube-web.desktop"
    install -m 0644 "${ROOT_DIR}/packaging/nanotube-web.desktop" "${APPDIR}/usr/share/applications/nanotube-web.desktop"
    install -m 0644 "${ROOT_DIR}/packaging/nanotube-web.svg" "${APPDIR}/nanotube-web.svg"
    install -m 0644 "${ROOT_DIR}/packaging/nanotube-web.svg" "${APPDIR}/.DirIcon"
    install -m 0644 "${ROOT_DIR}/packaging/nanotube-web.svg" "${APPDIR}/usr/share/icons/hicolor/scalable/apps/nanotube-web.svg"
    install -m 0755 "${ROOT_DIR}/packaging/appimage/AppRun" "${APPDIR}/AppRun"
    find "${APPDIR}" -exec touch -h -d "@${SOURCE_EPOCH}" {} +
}

main() {
    validate_inputs
    temporary_dir="$(mktemp -d "${TMPDIR:-/tmp}/nanotube-appimage.XXXXXXXX")"
    trap cleanup EXIT

    build_frontend
    build_binary "${temporary_dir}/nanotube-web"
    stage_appdir "${temporary_dir}/nanotube-web"
    bash "${ROOT_DIR}/packaging/appimage/test-appimage.sh" "${APPDIR}"

    printf '==> AppDir montado com sucesso em: %s\n' "${APPDIR}"
    if command -v appimagetool >/dev/null 2>&1; then
        local output_file="${ROOT_DIR}/build/NanoTube-Web-${VERSION}-x86_64.AppImage"
        printf '==> Gerando %s...\n' "${output_file}"
        appimagetool "${APPDIR}" "${output_file}"
        sha256sum "${output_file}" >"${output_file}.sha256"
        printf '==> AppImage e checksum gerados\n'
    else
        printf '==> appimagetool não instalado; AppDir pronto para empacotamento\n'
    fi
}

main "$@"
