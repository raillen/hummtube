---
id: flows-and-wireframes
status: canonical
---

# Fluxos de Navegação e Wireframes — NanoTube Web

## 1. Fluxo de Visualização e Detalhe de Vídeo
```text
[Grade de Vídeos / Home]
       │ (Clique no card ou Enter no D-Pad)
       ▼
[Modal de Player ou Vista Fullscreen]
       ├── Player de Vídeo Reativo
       ├── Controles de Transporte (Play, Seek, Volume, Speed, DSP)
       ├── Metadados (Título, Canal, Descrição expansível)
       ├── Painel de Ações (Favoritar, Adicionar à Playlist, Notas/Bookmarks, Copiar Link)
       └── Fila Lateral (Próximos vídeos)
```

## 2. Fluxo de Playlists e Regras Inteligentes
```text
[Página Playlists] ──► [Criar Playlist] ──► [Manual ou Inteligente]
                             │
                             ├─► Manual: Adiciona vídeos por drag-and-drop / menu do card
                             └─► Inteligente: Configura predicados (Canal = X E Duração < 10m)
```
