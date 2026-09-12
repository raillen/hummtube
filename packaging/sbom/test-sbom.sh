#!/usr/bin/env bash
set -euo pipefail

readonly SBOM_FILE="${1:-}"

[[ -n "${SBOM_FILE}" ]] || {
    printf 'uso: %s caminho-do-sbom.json\n' "$0" >&2
    exit 2
}
[[ -f "${SBOM_FILE}" ]] || {
    printf 'erro: SBOM não encontrado: %s\n' "${SBOM_FILE}" >&2
    exit 1
}
command -v node >/dev/null 2>&1 || {
    printf 'erro: node é obrigatório para validar o SBOM\n' >&2
    exit 1
}

node - "${SBOM_FILE}" <<'NODE'
const fs = require('fs');

const file = process.argv[2];
const bom = JSON.parse(fs.readFileSync(file, 'utf8'));
if (bom.bomFormat !== 'CycloneDX' || bom.specVersion !== '1.5') {
  throw new Error('formato CycloneDX 1.5 ausente');
}
if (!bom.metadata?.component || bom.metadata.component.name !== 'nanotube-web') {
  throw new Error('componente raiz NanoTube ausente');
}
if (!Array.isArray(bom.components) || bom.components.length === 0) {
  throw new Error('nenhum componente de dependência foi listado');
}
for (const component of bom.components) {
  if (!component.name || !component.version || !component.purl) {
    throw new Error('componente sem nome, versão ou purl');
  }
}
console.log(`==> SBOM validado: ${file} (${bom.components.length} componentes)`);
NODE
