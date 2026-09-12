# Entrypoint NanoIPTV (`cmd/nanoiptv/`)

## O que é este diretório?
Ponto de entrada do executável **NanoIPTV**.

## Para que serve?
Executável independente dentro da NanoSuite para reprodução de canais de IPTV ao vivo, parsing de listas M3U e guias eletrônicos de programação (EPG/XMLTV), isolado do NanoTube.

## Inventário
- `main.go`: Ponto de entrada e bootstrap dos serviços de IPTV.
