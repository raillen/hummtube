---
id: dependency-policy
status: canonical
---

# Política de Dependências — NanoTube Web

## Regras
1. **Backend**: Priorizar biblioteca padrão do Go e bibliotecas bem consolidadas (`wails/v3`, `modernc.org/sqlite`, `goose/v3`, `golang.org/x/*`). Evitar frameworks pesados ou ORMs opacos.
2. **Frontend**: Manter dependências enxutas (`svelte`, `tailwindcss`, `lucide-svelte`, `hls.js`). Evitar bibliotecas UI gigantescas que inflam o bundle.
