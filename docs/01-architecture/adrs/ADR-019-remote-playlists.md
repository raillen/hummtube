# ADR-019: Playlists Remotas como Port Separado

## Contexto
Listar itens de uma playlist remota do YouTube não deve misturar-se com o armazenamento de playlists locais nem baixar mídias desnecessariamente.

## Decisão
Criar o contrato `domain.RemotePlaylistProvider` desacoplado do catálogo sincronizado, permitindo visualizar playlists remotas diretamente na UI com paginação leve sob demanda.
