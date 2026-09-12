#!/usr/bin/env bash
set -euo pipefail

readonly ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
readonly PACKAGE_VERSION="${NANOTUBE_PACKAGE_VERSION:-0.1.0}"
readonly OUTPUT_DIR="${NANOTUBE_ARCH_OUTPUT_DIR:-${ROOT_DIR}/build}"
readonly SOURCE_TARBALL="${OUTPUT_DIR}/nanotube-web-${PACKAGE_VERSION}.tar.gz"

require_command() {
    command -v "$1" >/dev/null 2>&1 || {
        printf 'erro: comando obrigatório não encontrado: %s\n' "$1" >&2
        exit 1
    }
}

main() {
    require_command go
    require_command npm
    require_command makepkg
    require_command tar

    workdir="$(mktemp -d "${TMPDIR:-/tmp}/nanotube-arch.XXXXXXXX")"

    [[ "$(uname -m)" == x86_64 ]] || {
        printf 'erro: esta receita gera somente x86_64; host atual: %s\n' "$(uname -m)" >&2
        exit 1
    }

    printf '==> Compilando frontend NanoTube...\n'
    (cd "${ROOT_DIR}/frontend" && npm run build:nanotube)

    printf '==> Montando tarball de origem para makepkg...\n'
    mkdir -p "${OUTPUT_DIR}"
    source_staging="$(mktemp -d "${TMPDIR:-/tmp}/nanotube-arch-src.XXXXXXXX")"
    mkdir -p "${source_staging}/nanotube-web-${PACKAGE_VERSION}"
    cp -a "${ROOT_DIR}/." "${source_staging}/nanotube-web-${PACKAGE_VERSION}/"
    rm -rf "${source_staging}/nanotube-web-${PACKAGE_VERSION}/bin" \
           "${source_staging}/nanotube-web-${PACKAGE_VERSION}/build" \
           "${source_staging}/nanotube-web-${PACKAGE_VERSION}/frontend/node_modules" \
           "${source_staging}/nanotube-web-${PACKAGE_VERSION}/frontend/dist"
    tar -czf "${SOURCE_TARBALL}" -C "${source_staging}" "nanotube-web-${PACKAGE_VERSION}"
    rm -rf -- "${source_staging}"

    printf '==> Gerando pacote Arch com makepkg...\n'
    workdir="$(mktemp -d "${TMPDIR:-/tmp}/nanotube-arch.XXXXXXXX")"
    trap 'rm -rf -- "${workdir}"' EXIT
    cp "${ROOT_DIR}/packaging/arch/PKGBUILD" "${workdir}/PKGBUILD"
    cp "${SOURCE_TARBALL}" "${workdir}/nanotube-web-${PACKAGE_VERSION}.tar.gz"
    sed -i "s/^pkgver=.*/pkgver=${PACKAGE_VERSION}/" "${workdir}/PKGBUILD"
    (cd "${workdir}" && makepkg -f --noconfirm)
    cp "${workdir}"/nanotube-web-*.pkg.tar.zst "${OUTPUT_DIR}/"
    sha256sum "${OUTPUT_DIR}"/nanotube-web-*.pkg.tar.zst >"${OUTPUT_DIR}/nanotube-web-arch.sha256"

    printf '==> Pacote Arch gerado em: %s\n' "${OUTPUT_DIR}"
}

main "$@"
