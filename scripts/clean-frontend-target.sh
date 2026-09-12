#!/usr/bin/env bash
set -euo pipefail

target="${1:-}"
case "${target}" in
  hummtube|hummiptv|hummmusic|nanotube|nanoiptv|nanomusic) ;;
  *)
    printf 'uso: %s {hummtube|hummiptv|hummmusic|nanotube|nanoiptv|nanomusic}\n' "$0" >&2
    exit 2
    ;;
esac

root_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
output_dir="${root_dir}/frontend/dist/${target}"

if [[ -L "${output_dir}" ]]; then
  printf 'erro: destino do frontend não pode ser link simbólico: %s\n' "${output_dir}" >&2
  exit 1
fi

mkdir -p "${output_dir}"
# O marcador mantém o go:embed compilável antes do primeiro build nativo.
find "${output_dir}" -mindepth 1 ! -name '.gitkeep' -delete
