---
id: thumbnails-and-cache
status: canonical
---

# Cache de Miniaturas — NanoTube Web

## 1. Armazenamento em Disco
- Localização: `~/.cache/nanotube-web/thumbnails/`;
- Nomeação: Hash SHA256 da URL original da miniatura;
- Servidor Local: Wails Asset Server serve os arquivos diretamente para as tags `<img>` no Svelte via protocolo local `wails://nanotube/thumbnail?hash=...` ou rota local.

## 2. Política de Poda
- O cache é limitado a 250 MB por padrão;
- A poda remove os arquivos menos recentemente acessados (LRU).
