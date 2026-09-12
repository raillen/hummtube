---
id: recommendations
status: canonical
---

# Motor de Recomendações v2 (MMR & Afinidades)

## 1. Algoritmo Determinístico e Explicável

O `RecommendationEngine` v2 não usa modelos LLM nem inferência pesada. Ele calcula um score composto para cada vídeo candidato:

$$Score(V) = W_{topic} \cdot A_{topic} + W_{channel} \cdot A_{channel} + W_{fresh} \cdot F(t) + W_{watch} \cdot (1 - P_{completed}) + W_{novelty} \cdot N(V) - \lambda \cdot Sat(Channel)$$

### Pesos Calibrados:
- $W_{topic} = 0.30$: Afinidade com tópicos de interesse;
- $W_{channel} = 0.30$: Engajamento com o canal (favorito, histórico, frequência);
- $W_{fresh} = 0.15$: Frescor e novidade temporal (decaimento exponencial);
- $W_{watch} = 0.10$: Vídeos não finalizados ou novos uploads;
- $W_{novelty} = 0.10$: Descoberta de canais inscritos menos frequentes;
- $\lambda \cdot Sat(Channel) = 0.05$: Penalidade de saturação MMR para garantir diversidade na grade.
