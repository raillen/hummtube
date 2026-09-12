#!/usr/bin/env bash
set -euo pipefail

readonly ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
readonly VERSION="${NANOTUBE_PACKAGE_VERSION:-0.1.0}"
readonly OUTPUT_DIR="${NANOTUBE_SBOM_OUTPUT_DIR:-${ROOT_DIR}/build/sbom}"
readonly OUTPUT_FILE="${NANOTUBE_SBOM_OUTPUT_FILE:-${OUTPUT_DIR}/nanotube-web-${VERSION}.cdx.json}"
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

validate_version() {
    [[ "${VERSION}" =~ ^[0-9][0-9A-Za-z.+:~_-]*$ ]] || {
        printf 'erro: versão inválida para o SBOM: %s\n' "${VERSION}" >&2
        exit 1
    }
}

main() {
    require_command go
    require_command node
    validate_version

    temporary_dir="$(mktemp -d "${TMPDIR:-/tmp}/nanotube-sbom.XXXXXXXX")"
    trap cleanup EXIT

    (
        cd "${ROOT_DIR}"
        go list -m -f '{{.Path}}\t{{.Version}}' all >"${temporary_dir}/go-modules.tsv"
    )

    mkdir -p "${OUTPUT_DIR}"
node - "${ROOT_DIR}" "${temporary_dir}/go-modules.tsv" "${OUTPUT_FILE}" "${VERSION}" <<'NODE'
const crypto = require('crypto');
const fs = require('fs');
const path = require('path');

const [rootDir, goModulesPath, outputPath, version] = process.argv.slice(2);
const lockPath = path.join(rootDir, 'frontend', 'package-lock.json');
const lock = JSON.parse(fs.readFileSync(lockPath, 'utf8'));
const components = [];
const seen = new Set();

function addComponent(component) {
  if (!component['bom-ref'] || seen.has(component['bom-ref'])) return;
  seen.add(component['bom-ref']);
  components.push(component);
}

function npmRef(name, packageVersion) {
  return `pkg:npm/${encodeURIComponent(name)}@${encodeURIComponent(packageVersion)}`;
}

for (const [packagePath, metadata] of Object.entries(lock.packages || {})) {
  if (!packagePath.startsWith('node_modules/') || !metadata.version) continue;
  const name = metadata.name || packagePath.slice('node_modules/'.length);
  const ref = npmRef(name, metadata.version);
  addComponent({
    type: 'library',
    group: 'npm',
    name,
    version: metadata.version,
    purl: ref,
    'bom-ref': ref,
  });
}

for (const line of fs.readFileSync(goModulesPath, 'utf8').split('\n')) {
  if (!line.trim()) continue;
  const [modulePath, moduleVersion] = line.split('\t');
  if (!modulePath || !moduleVersion) continue;
  const ref = `pkg:golang/${encodeURIComponent(modulePath)}@${encodeURIComponent(moduleVersion)}`;
  addComponent({
    type: 'library',
    group: 'golang',
    name: modulePath,
    version: moduleVersion,
    purl: ref,
    'bom-ref': ref,
  });
}

const rootRef = `pkg:generic/nanotube-web@${encodeURIComponent(version)}`;
const serialHash = crypto.createHash('sha256').update(rootRef).digest('hex').slice(0, 32);
const serialNumber = `urn:uuid:${serialHash.slice(0, 8)}-${serialHash.slice(8, 12)}-${serialHash.slice(12, 16)}-${serialHash.slice(16, 20)}-${serialHash.slice(20)}`;
const bom = {
  bomFormat: 'CycloneDX',
  specVersion: '1.5',
  serialNumber,
  version: 1,
  metadata: {
    component: {
      type: 'application',
      name: 'nanotube-web',
      version,
      'bom-ref': rootRef,
      description: 'NanoTube Web source and build dependencies',
    },
    properties: [
      { name: 'org.nanotube.scope', value: 'source-dependencies' },
      { name: 'org.nanotube.generated-by', value: 'packaging/sbom/generate-sbom.sh' },
    ],
  },
  components,
};

fs.writeFileSync(outputPath, `${JSON.stringify(bom, null, 2)}\n`, { mode: 0o644 });
console.log(`==> SBOM CycloneDX gerado: ${outputPath}`);
console.log(`==> Componentes listados: ${components.length}`);
NODE
}

main "$@"
