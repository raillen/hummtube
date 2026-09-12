# ADR-016: Explicit Yt-Dlp com Attestation de PO Token

## Contexto
O YouTube exige Proof of Origin (PO) tokens e desafios JavaScript para liberar streams de alta qualidade sem erros de HTTP 403.

## Decisão
Utilizar o `ExplicitYtDlpResolver` configurado com `--extractor-args "youtube:player_client=android,ios,mweb"` e integração opcional com PO Token Provider (`bgutil-ytdlp-pot-provider`).
