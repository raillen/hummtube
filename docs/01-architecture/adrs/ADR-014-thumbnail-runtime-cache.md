# ADR-014: Cache de Miniaturas e Servidor de Assets

## Contexto
O frontend precisa carregar miniaturas com rapidez sem re-baixar da internet nem sobrecarregar o tráfego externo.

## Decisão
O backend Go expõe um AssetHandler local embutido no Wails v3 (`wails://nanotube/thumbnail?id=...` ou rota local) que serve miniaturas diretamente do cache em disco content-addressed.
