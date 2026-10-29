---
name: docker-multistage
description: Imagen Docker multi-etapa
---
# docker-multistage

> Rector: `.specify/memory/constitution.md`. Ante conflicto, manda la constitución.

## What I do
- Etapa de build con `golang`, etapa final mínima, usuario sin privilegios, sin secretos ni `.env`.

## Verify
- `docker build` OK y la imagen arranca con variables de entorno.
