# ADR-005: Home Local Baseada em Inscrições e Afinidades (Sem LLM)

## Contexto
O algoritmo do YouTube oficial incentiva clickbaits, retenção artificial e distração.

## Decisão
Construir a Home de forma 100% local e determinística usando o **Recommendation Engine v2** com algoritmo MMR guloso (Maximal Marginal Relevance), afinidade de tópicos/canais e decaimento temporal, sem dependência de modelos de linguagem (LLM) no runtime.
