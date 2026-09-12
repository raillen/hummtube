#!/usr/bin/env bash
set -euo pipefail

readonly ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
readonly PACKAGE_VERSION="${NANOTUBE_PACKAGE_VERSION:-0.1.0}"
readonly OUTPUT_DIR="${NANOTUBE_RPM_OUTPUT_DIR:-${ROOT_DIR}/build}"

require_command() {
    command -v "$1" >/dev/null 2>&1 || {
        printf 'erro: comando obrigatório não encontrado: %s\n' "$1" >&2
        exit 1
    }
}

main() {
    require_command go
    require_command npm
    require_command rpmbuild
    require_command tar

    printf '==> Compilando frontend NanoTube...\n'
    (cd "${ROOT_DIR}/frontend" && npm run build:nanotube)

    printf '==> Montando árvore rpmbuild...\n'
    workdir="$(mktemp -d "${TMPDIR:-/tmp}/nanotube-rpm.XXXXXXXX")"
    trap 'rm -rf -- "${workdir}"' EXIT
    mkdir -p "${workdir}"/{BUILD,RPMS,SOURCES,SPECS,SRPMS}
    source_staging="$(mktemp -d "${TMPDIR:-/tmp}/nanotube-rpm-src.XXXXXXXX")"
    mkdir -p "${source_staging}/nanotube-web-${PACKAGE_VERSION}"
    cp -a "${ROOT_DIR}/." "${source_staging}/nanotube-web-${PACKAGE_VERSION}/"
    rm -rf "${source_staging}/nanotube-web-${PACKAGE_VERSION}/bin" \
           "${source_staging}/nanotube-web-${PACKAGE_VERSION}/build" \
           "${source_staging}/nanotube-web-${PACKAGE_VERSION}/frontend/node_modules" \
           "${source_staging}/nanotube-web-${PACKAGE_VERSION}/frontend/dist"
    tar -czf "${workdir}/SOURCES/nanotube-web-${PACKAGE_VERSION}.tar.gz" \
        -C "${source_staging}" "nanotube-web-${PACKAGE_VERSION}"
    rm -rf -- "${source_staging}"
    sed "s/^Version:.*/Version:        ${PACKAGE_VERSION}/" \
        "${ROOT_DIR}/packaging/rpm/nanotube-web.spec" >"${workdir}/SPECS/nanotube-web.spec"

    printf '==> Gerando RPM com rpmbuild...\n'
    rpmbuild --define "_topdir ${workdir}" -bb "${workdir}/SPECS/nanotube-web.spec"
    mkdir -p "${OUTPUT_DIR}"
    cp "${workdir}"/RPMS/*/*.rpm "${OUTPUT_DIR}/"
    (cd "${OUTPUT_DIR}" && sha256sum nanotube-web-*.rpm >nanotube-web-rpm.sha256)

    printf '==> RPM gerado em: %s\n' "${OUTPUT_DIR}"
    if command -v rpmlint >/dev/null 2>&1; then
        rpmlint "${OUTPUT_DIR}"/nanotube-web-*.rpm || printf 'aviso: rpmlint reportou problemas\n'
    fi
}

main "$@"
