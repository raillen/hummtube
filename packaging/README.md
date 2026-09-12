# Empacotamento e Distribuição Linux (`packaging/`)

## O que é este diretório?
Scripts, manifests e especificações de empacotamento multi-distribuição para Linux e Windows.

## Para que serve?
Gera pacotes nativos para Debian/Ubuntu (`.deb`), Arch Linux (`PKGBUILD`), Fedora (`.rpm`), AppImage universal e instaladores Windows.

## Inventário
- `deb/`: Arquivos de controle Debian;
- `arch/`: PKGBUILD para pacotes Arch Linux;
- `rpm/`: Arquivo spec para empacotamento RPM;
- `appimage/`: Receita e AppRun para geração de AppImage;
- `windows/`: Scripts de cross-compilação Windows;
- `sbom/`: Geração de inventário SBOM (CycloneDX);
- `check-tools.sh`: Verificador de ferramentas de build do sistema.
