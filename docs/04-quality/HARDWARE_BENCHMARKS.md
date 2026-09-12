---
id: hardware-benchmarks
status: canonical
---

# Benchmarks em Hardware de Referência — NanoTube Web

## Ambiente de Referência
- CPU: Intel Celeron 1037U / Dual-Core 1.8GHz;
- RAM: ~1.8 GB RAM disponível;
- SO: Arch Linux / Debian 12 com X11 / Wayland.

## Metodologia
- Medição de RSS (`ps aux` / memory profiling);
- Medição de tempo de renderização inicial com Chrome DevTools / WebView Inspector;
- Testes com `benchstat` comparando melhorias no banco SQLite e algoritmo de recomendação.
- Três execuções de aquecimento e pelo menos 30 execuções medidas para mediana, p95 e dispersão de cenários locais;
- Pelo menos 20 amostras distribuídas no tempo para latência remota, separando DNS, conexão, provider, enriquecimento e renderização;
- Registro de versão do NanoTube, Playback Runtime, WebKitGTK, kernel, driver gráfico e sessão X11/Wayland;
- Medição da árvore de processos, incluindo WebKit e GPU, além do processo Go.
