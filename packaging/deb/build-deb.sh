#!/usr/bin/env bash
set -euo pipefail

readonly ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
readonly PACKAGE_NAME="nanotube-web"
readonly PACKAGE_VERSION="${NANOTUBE_PACKAGE_VERSION:-0.1.0}"
readonly PACKAGE_ARCHITECTURE="amd64"
readonly SOURCE_EPOCH="${SOURCE_DATE_EPOCH:-946684800}"
readonly OUTPUT_DIR="${NANOTUBE_DEB_OUTPUT_DIR:-${ROOT_DIR}/build}"
readonly OUTPUT_FILE="${OUTPUT_DIR}/${PACKAGE_NAME}_${PACKAGE_VERSION}_${PACKAGE_ARCHITECTURE}.deb"
temporary_dir=""

cleanup() {
    if [[ -n "${temporary_dir}" ]]; then
        rm -rf -- "${temporary_dir}"
    fi
}

require_command() {
    local command_name="$1"
    if ! command -v "${command_name}" >/dev/null 2>&1; then
        printf 'erro: comando obrigatório não encontrado: %s\n' "${command_name}" >&2
        exit 1
    fi
}

validate_build_inputs() {
    case "$(uname -m)" in
        x86_64|amd64) ;;
        *)
            printf 'erro: esta preview gera somente amd64; host atual: %s\n' "$(uname -m)" >&2
            exit 1
            ;;
    esac

    if [[ ! "${PACKAGE_VERSION}" =~ ^[0-9][0-9A-Za-z.+:~_-]*$ ]]; then
        printf 'erro: versão Debian inválida: %s\n' "${PACKAGE_VERSION}" >&2
        exit 1
    fi
    if [[ ! "${SOURCE_EPOCH}" =~ ^[0-9]+$ ]]; then
        printf 'erro: SOURCE_DATE_EPOCH deve ser um inteiro não negativo\n' >&2
        exit 1
    fi

    require_command ar
    require_command awk
    require_command du
    require_command find
    require_command go
    require_command gzip
    require_command install
    require_command mktemp
    require_command npm
    require_command sed
    require_command sha256sum
    require_command tar
    require_command touch
}

build_nanotube_frontend() {
    local nanotube_dist="${ROOT_DIR}/frontend/dist/nanotube"

    printf '==> Compilando somente o frontend NanoTube...\n'
    if [[ -L "${nanotube_dist}" ]]; then
        printf 'erro: o destino do frontend não pode ser um link simbólico\n' >&2
        exit 1
    fi
    mkdir -p "${nanotube_dist}"
    # Vite preserva o diretório por causa do marcador go:embed; remova apenas
    # artefatos gerados do target NanoTube para não empacotar chunks obsoletos.
    find "${nanotube_dist}" -mindepth 1 ! -name '.gitkeep' -delete
    (
        cd "${ROOT_DIR}/frontend"
        npm run build:nanotube
    )
    if [[ ! -f "${ROOT_DIR}/frontend/dist/nanotube/index.html" ]]; then
        printf 'erro: frontend NanoTube não foi gerado\n' >&2
        exit 1
    fi
}

build_nanotube_binary() {
    local temporary_dir="$1"
    local binary_path="$2"
    local overlay_source="${temporary_dir}/assets.nanotube.go"
    local overlay_config="${temporary_dir}/overlay.json"

    # O overlay limita o go:embed ao target NanoTube sem alterar o workspace e
    # sem carregar os bundles NanoIPTV/NanoMusic para o executável da preview.
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

    printf '==> Compilando o executável NanoTube amd64...\n'
    (
        cd "${ROOT_DIR}"
        CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go build \
            -buildvcs=false \
            -trimpath \
            -overlay "${overlay_config}" \
            -ldflags "-s -w -buildid= -X github.com/nanotube/nanotube-web/internal/diagnostics.Version=${PACKAGE_VERSION}" \
            -o "${binary_path}" \
            ./cmd/nanotube-web
    )
}

render_control_file() {
    local package_root="$1"
    local installed_size
    installed_size="$(du -sk "${package_root}/usr" | awk '{print $1}')"

    sed \
        -e "s/^Version:.*/Version: ${PACKAGE_VERSION}/" \
        -e "s/^Architecture:.*/Architecture: ${PACKAGE_ARCHITECTURE}/" \
        "${ROOT_DIR}/packaging/deb/control" >"${package_root}/DEBIAN/control"
    printf 'Installed-Size: %s\n' "${installed_size}" >>"${package_root}/DEBIAN/control"
}

stage_package() {
    local temporary_dir="$1"
    local package_root="${temporary_dir}/package-root"
    local binary_path="${temporary_dir}/nanotube-web"

    mkdir -p \
        "${package_root}/DEBIAN" \
        "${package_root}/usr/bin" \
        "${package_root}/usr/share/applications" \
        "${package_root}/usr/share/icons/hicolor/scalable/apps"

    build_nanotube_binary "${temporary_dir}" "${binary_path}"
    install -m 0755 "${binary_path}" "${package_root}/usr/bin/nanotube-web"
    install -m 0644 \
        "${ROOT_DIR}/packaging/deb/nanotube-web.desktop" \
        "${package_root}/usr/share/applications/nanotube-web.desktop"
    install -m 0644 \
        "${ROOT_DIR}/packaging/nanotube-web.svg" \
        "${package_root}/usr/share/icons/hicolor/scalable/apps/nanotube-web.svg"
    render_control_file "${package_root}"

    find "${package_root}" -type d -exec chmod 0755 {} +
    chmod 0644 "${package_root}/DEBIAN/control"
    find "${package_root}" -exec touch -h -d "@${SOURCE_EPOCH}" {} +
}

create_debian_archive() {
    local temporary_dir="$1"
    local package_root="${temporary_dir}/package-root"
    local archive_root="${temporary_dir}/archive"
    local temporary_output="${temporary_dir}/${PACKAGE_NAME}.deb"

    mkdir -p "${archive_root}"
    printf '2.0\n' >"${archive_root}/debian-binary"
    tar \
        --sort=name \
        --mtime="@${SOURCE_EPOCH}" \
        --owner=0 --group=0 --numeric-owner \
        -C "${package_root}/DEBIAN" \
        -cf - . | gzip -n -9 >"${archive_root}/control.tar.gz"
    tar \
        --sort=name \
        --mtime="@${SOURCE_EPOCH}" \
        --owner=0 --group=0 --numeric-owner \
        --exclude='./DEBIAN' \
        -C "${package_root}" \
        -cf - . | gzip -n -9 >"${archive_root}/data.tar.gz"
    chmod 0644 \
        "${archive_root}/debian-binary" \
        "${archive_root}/control.tar.gz" \
        "${archive_root}/data.tar.gz"

    (
        cd "${archive_root}"
        ar rcsD "${temporary_output}" debian-binary control.tar.gz data.tar.gz
    )
    mkdir -p "${OUTPUT_DIR}"
    mv -f "${temporary_output}" "${OUTPUT_FILE}"
    chmod 0644 "${OUTPUT_FILE}"
}

main() {
    validate_build_inputs

    temporary_dir="$(mktemp -d "${TMPDIR:-/tmp}/nanotube-deb.XXXXXXXX")"
    trap cleanup EXIT

    build_nanotube_frontend
    stage_package "${temporary_dir}"
    create_debian_archive "${temporary_dir}"
    "${ROOT_DIR}/packaging/deb/test-package.sh" "${OUTPUT_FILE}"
    (
        cd "${OUTPUT_DIR}"
        sha256sum "$(basename "${OUTPUT_FILE}")" >"$(basename "${OUTPUT_FILE}").sha256"
    )

    printf '==> Pacote Debian gerado: %s\n' "${OUTPUT_FILE}"
    printf '==> Checksum SHA-256: %s.sha256\n' "${OUTPUT_FILE}"
}

main "$@"
