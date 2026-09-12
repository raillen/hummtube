# Motor de Playback (`internal/playback/`)

## O que é este diretório?
Contém o `PlaybackResolver` e estratégias em cascata para obtenção de streams de áudio e vídeo do YouTube.

## Para que serve?
Executa a resolução resiliente de streams utilizando yt-dlp explícito, PO Token Attestation Providers e instâncias Invidious de contingência com guardas anti-SSRF.
