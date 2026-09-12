# ADR-006: Desacoplamento via PlaybackResolver

## Contexto
Mudanças no YouTube e quebras em extratores não devem impactar a interface ou o modelo de domínio.

## Decisão
Toda resolução de URL e streams passa pela interface `PlaybackResolver`. A UI requisita um `PlaybackPlan` estruturado (streams de vídeo/áudio, headers HTTP necessários, legendas) e ignora os detalhes de extração.
