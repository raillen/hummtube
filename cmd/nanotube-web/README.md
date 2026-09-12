# Entrypoint NanoTube Web (`cmd/nanotube-web/`)

## O que é este diretório?
Ponto de entrada do executável principal do **NanoTube Web**.

## Para que serve?
Inicializa a ponte de serviços Wails v3, abre o banco de dados `nanotube-web.db`, configura o `PlaybackResolver`, inicializa a janela desktop nativa e monta a SPA do NanoTube.

## Inventário
- `main.go`: Ponto de entrada (`func main()`), inicialização de dependências e loop de eventos Wails v3.
