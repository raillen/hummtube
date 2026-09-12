#!/usr/bin/env bash
set -euo pipefail

root_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
output_dir="${NANOTUBE_BENCH_OUTPUT_DIR:-${root_dir}/build/benchmarks}"
timestamp="$(date -u +%Y%m%dT%H%M%SZ)"
report="${output_dir}/nanotube-${timestamp}.txt"

mkdir -p "${output_dir}"
{
  printf 'NanoTube local performance benchmark\n'
  printf 'timestamp_utc=%s\n' "${timestamp}"
  printf 'host=%s\n' "$(uname -a)"
  printf 'session=%s\n' "${XDG_SESSION_TYPE:-unknown}"
  printf 'display=%s\n' "${DISPLAY:-${WAYLAND_DISPLAY:-none}}"
  printf '\n[go benchmarks]\n'
  if command -v go >/dev/null 2>&1; then
    (cd "${root_dir}" && go test -run '^$' -bench . -benchmem ./internal/...)
  else
    printf 'go=missing\n'
  fi
  printf '\n[frontend bundle]\n'
  if [[ -d "${root_dir}/frontend/dist/nanotube" ]]; then
    du -ah "${root_dir}/frontend/dist/nanotube" | sort -h | tail -n 20
  else
    printf 'frontend_dist=missing (execute task frontend:build first)\n'
  fi
  printf '\n[process tools]\n'
  for tool in ps smem /usr/bin/time; do
    if command -v "${tool}" >/dev/null 2>&1 || [[ -x "${tool}" ]]; then
      printf '%s=available\n' "${tool}"
    else
      printf '%s=missing\n' "${tool}"
    fi
  done
} | tee "${report}"

printf '\nRelatório salvo em: %s\n' "${report}"
