#!/usr/bin/env bash
set -euo pipefail

readonly PACKAGE_ARGUMENT="${1:-}"
PACKAGE_FILE=""
CONTROL_VERSION=""
extraction_dir=""

cleanup() {
    if [[ -n "${extraction_dir}" ]]; then
        rm -rf -- "${extraction_dir}"
    fi
}

fail() {
    printf 'erro de validação do pacote: %s\n' "$1" >&2
    exit 1
}

require_command() {
    local command_name="$1"
    command -v "${command_name}" >/dev/null 2>&1 || fail "comando ausente: ${command_name}"
}

validate_archive_members() {
    local members
    members="$(ar t "${PACKAGE_FILE}")"
    [[ "${members}" == $'debian-binary\ncontrol.tar.gz\ndata.tar.gz' ]] || \
        fail "membros ar inesperados"
}

validate_control() {
    local extraction_dir="$1"
    local control_file="${extraction_dir}/control/control"

    mkdir -p "${extraction_dir}/control"
    tar -xzf "${extraction_dir}/control.tar.gz" -C "${extraction_dir}/control"
    [[ -f "${control_file}" ]] || fail "DEBIAN/control ausente"
    grep -qx 'Package: nanotube-web' "${control_file}" || fail "nome do pacote inválido"
    grep -qx 'Architecture: amd64' "${control_file}" || fail "arquitetura Debian inválida"
    grep -qx 'Depends: libc6 (>= 2.38), ca-certificates, libgtk-4-1, libwebkitgtk-6.0-4' "${control_file}" || \
        fail "dependência glibc/GTK/WebKit diverge da ABI desta preview"
    grep -q 'libgtk-4-1' "${control_file}" || fail "dependência GTK 4 ausente"
    grep -q 'libwebkitgtk-6.0-4' "${control_file}" || fail "dependência WebKitGTK 6 ausente"
    grep -q '^Recommends: .*yt-dlp' "${control_file}" || fail "yt-dlp não declarado"
    CONTROL_VERSION="$(awk '/^Version: / { print $2; exit }' "${control_file}")"
    [[ -n "${CONTROL_VERSION}" ]] || fail "versão Debian ausente"
}

validate_data() {
    local extraction_dir="$1"
    local data_dir="${extraction_dir}/data"
    local executable="${data_dir}/usr/bin/nanotube-web"
    local packaged_files

    mkdir -p "${data_dir}"
    tar -xzf "${extraction_dir}/data.tar.gz" -C "${data_dir}"

    [[ -x "${executable}" ]] || fail "executável NanoTube ausente"
    [[ -f "${data_dir}/usr/share/applications/nanotube-web.desktop" ]] || \
        fail "desktop entry ausente"
    [[ -f "${data_dir}/usr/share/icons/hicolor/scalable/apps/nanotube-web.svg" ]] || \
        fail "ícone ausente"
    [[ ! -e "${data_dir}/usr/bin/nanoiptv" ]] || fail "NanoIPTV incluído no pacote"
    [[ ! -e "${data_dir}/usr/bin/nanomusic" ]] || fail "NanoMusic incluído no pacote"
    [[ ! -e "${data_dir}/usr/share/nanotube-web/frontend/dist/nanoiptv" ]] || \
        fail "bundle NanoIPTV incluído no pacote"
    [[ ! -e "${data_dir}/usr/share/nanotube-web/frontend/dist/nanomusic" ]] || \
        fail "bundle NanoMusic incluído no pacote"

    packaged_files="$(find "${data_dir}" -type f -printf '%P\n' | sort)"
    [[ "${packaged_files}" == $'usr/bin/nanotube-web\nusr/share/applications/nanotube-web.desktop\nusr/share/icons/hicolor/scalable/apps/nanotube-web.svg' ]] || \
        fail "o pacote contém arquivos fora da allowlist NanoTube"

    file "${executable}" | grep -q 'x86-64' || fail "ELF não é x86-64"
    readelf -d "${executable}" | grep -q 'libwebkitgtk-6.0.so.4' || \
        fail "ELF não referencia WebKitGTK 6"
    readelf -d "${executable}" | grep -q 'libgtk-4.so.1' || \
        fail "ELF não referencia GTK 4"
    [[ "$("${executable}" --version)" == "NanoTube v${CONTROL_VERSION}" ]] || \
        fail "versão do executável diverge do pacote"

    if command -v desktop-file-validate >/dev/null 2>&1; then
        desktop-file-validate "${data_dir}/usr/share/applications/nanotube-web.desktop"
    fi
}

main() {
    [[ -n "${PACKAGE_ARGUMENT}" ]] || fail "informe o caminho do .deb"
    [[ -f "${PACKAGE_ARGUMENT}" ]] || fail "arquivo não encontrado: ${PACKAGE_ARGUMENT}"
    PACKAGE_FILE="$(cd "$(dirname "${PACKAGE_ARGUMENT}")" && pwd)/$(basename "${PACKAGE_ARGUMENT}")"
    require_command ar
    require_command awk
    require_command file
    require_command find
    require_command grep
    require_command readelf
    require_command sort
    require_command tar

    extraction_dir="$(mktemp -d "${TMPDIR:-/tmp}/nanotube-deb-test.XXXXXXXX")"
    trap cleanup EXIT

    (
        cd "${extraction_dir}"
        ar x "${PACKAGE_FILE}"
    )
    [[ "$(cat "${extraction_dir}/debian-binary")" == '2.0' ]] || \
        fail "versão do formato Debian inválida"
    validate_archive_members
    validate_control "${extraction_dir}"
    validate_data "${extraction_dir}"

    if command -v dpkg-deb >/dev/null 2>&1; then
        dpkg-deb --info "${PACKAGE_FILE}" >/dev/null
        dpkg-deb --contents "${PACKAGE_FILE}" >/dev/null
    fi
    printf '==> Pacote Debian validado: %s\n' "${PACKAGE_FILE}"
}

main "$@"
