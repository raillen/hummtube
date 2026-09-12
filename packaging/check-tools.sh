#!/usr/bin/env bash
set -euo pipefail

# Verifica as ferramentas necessárias para cada operação sem transformar
# dependências opcionais em uma falha de build. O objetivo é dar uma mensagem
# acionável antes de um comando longo começar.
target="${1:-verify}"
missing=0

tool_status() {
    local command_name="$1"
    local requirement="$2"

    if command -v "${command_name}" >/dev/null 2>&1; then
        printf 'ok: %-24s (%s)\n' "${command_name}" "${requirement}"
        return
    fi

    if [[ "${requirement}" == optional ]]; then
        printf 'aviso: %-20s ausente (opcional)\n' "${command_name}"
        return
    fi

    printf 'erro: %-24s ausente (obrigatório para %s)\n' "${command_name}" "${target}" >&2
    missing=$((missing + 1))
}

check_tools() {
    local requirement="$1"
    shift
    local command_name
    for command_name in "$@"; do
        tool_status "${command_name}" "${requirement}"
    done
}

printf '==> Verificando ferramentas para: %s\n' "${target}"
case "${target}" in
    verify)
        check_tools required go node npm
        check_tools required ar awk file find grep readelf sort tar
        ;;
    deb)
        check_tools required go node npm ar awk du find gzip install mktemp sed sha256sum tar
        check_tools required file grep readelf sort
        check_tools optional desktop-file-validate dpkg-deb lintian
        ;;
    appimage)
        check_tools required go node npm find grep mktemp readelf file
        check_tools optional appimagetool desktop-file-validate
        ;;
    sbom)
        check_tools required go node npm
        ;;
    rpm)
        check_tools required go node npm rpmbuild
        check_tools optional desktop-file-validate rpmlint
        ;;
    arch)
        check_tools required go node npm makepkg
        check_tools optional namcap
        ;;
    windows)
        check_tools required go node npm
        check_tools optional zip
        ;;
    dev)
        check_tools required go node npm wails3
        ;;
    *)
        printf 'erro: alvo desconhecido: %s\n' "${target}" >&2
        printf 'uso: %s {verify|deb|appimage|sbom|rpm|arch|windows|dev}\n' "$0" >&2
        exit 2
        ;;
esac

if (( missing > 0 )); then
    printf 'erro: %d ferramenta(s) obrigatória(s) ausente(s)\n' "${missing}" >&2
    exit 1
fi

printf '==> Ferramentas obrigatórias disponíveis para %s\n' "${target}"
