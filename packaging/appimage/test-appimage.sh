#!/usr/bin/env bash
set -euo pipefail

readonly TARGET="${1:-build/AppDir}"

fail() {
    printf 'erro de validação AppImage: %s\n' "$1" >&2
    exit 1
}

require_command() {
    command -v "$1" >/dev/null 2>&1 || fail "comando ausente: $1"
}

validate_appdir() {
    local appdir="$1"
    local executable="${appdir}/usr/bin/nanotube-web"
    local desktop_entry="${appdir}/usr/share/applications/nanotube-web.desktop"
    local icon="${appdir}/usr/share/icons/hicolor/scalable/apps/nanotube-web.svg"
    local frontend_index="${appdir}/usr/share/nanotube-web/frontend/dist/nanotube/index.html"

    [[ -d "${appdir}" ]] || fail "AppDir não encontrado: ${appdir}"
    [[ -x "${appdir}/AppRun" ]] || fail "AppRun ausente ou não executável"
    [[ -x "${executable}" ]] || fail "executável NanoTube ausente ou não executável"
    [[ -f "${desktop_entry}" ]] || fail "desktop entry ausente"
    [[ -f "${icon}" ]] || fail "ícone ausente"
    [[ -f "${frontend_index}" ]] || fail "frontend NanoTube ausente"

    find "${appdir}" -type l -print -quit | grep -q . && \
        fail "AppDir não pode conter links simbólicos não auditados"
    find "${appdir}" \( -iname '*nanoiptv*' -o -iname '*nanomusic*' \) -print -quit | grep -q . && \
        fail "AppDir contém artefatos do NanoIPTV ou NanoMusic"

    file "${executable}" | grep -Eq 'ELF 64-bit.*x86-64|ELF 64-bit.*AMD' || \
        fail "executável não é um ELF x86-64"
    # O linker externo usado pelo Wails/GTK pode produzir EXEC ou DYN
    # dependendo da toolchain. Ambos são ELF executáveis válidos para esta
    # validação estrutural; a matriz de distribuição continua responsável por
    # verificar as políticas de hardening da distro.
    # LC_ALL=C garante saída em inglês independente do locale do host.
    LC_ALL=C readelf -h "${executable}" | grep -Eq 'Type:[[:space:]]*(EXEC|DYN)' || \
        fail "executável ELF inválido"

    if command -v desktop-file-validate >/dev/null 2>&1; then
        desktop-file-validate "${appdir}/nanotube-web.desktop"
        desktop-file-validate "${desktop_entry}"
    fi
}

validate_appimage() {
    local image="$1"
    [[ -f "${image}" ]] || fail "AppImage não encontrado: ${image}"
    [[ -x "${image}" ]] || fail "AppImage não é executável"
    file "${image}" | grep -Eq 'ELF 64-bit.*x86-64|ELF 64-bit.*AMD' || \
        fail "artefato não parece ser um AppImage ELF x86-64"
}

require_command file
require_command find
require_command grep
require_command readelf

if [[ -d "${TARGET}" ]]; then
    validate_appdir "${TARGET}"
elif [[ -f "${TARGET}" ]]; then
    validate_appimage "${TARGET}"
else
    fail "caminho não encontrado: ${TARGET}"
fi

printf '==> AppImage/AppDir validado: %s\n' "${TARGET}"
