# Entrypoints Executáveis (`cmd/`)

## O que é este diretório?
Contém os pontos de entrada `main` (`package main`) de todos os aplicativos executáveis do repositório.

## Para que serve?
Implementa a inicialização do runtime Wails v3, configuração dos serviços de aplicação e dispatch de janelas nativas conforme ADR-022.

## Inventário
- `nanotube-web/`: Aplicativo desktop principal (cliente YouTube leve);
- `nanoiptv/`: Aplicativo desktop dedicado para IPTV e EPG (M3U/XMLTV);
- `nanomusic/`: Aplicativo desktop focado na experiência musical com YouTube Music.
