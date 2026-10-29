---
name: 03-api-base
description: Reglas base de la API
---
# 03-api-base

> Rector: `.specify/memory/constitution.md`. Ante conflicto, manda la constitución.

## What I do
- Rutas `/api/v1`, sin versión → 404. JSON `snake_case`. Errores `{ codigo, mensaje, campos[], detalle? }` con `detalle` solo no-prod.
- Paginación por cursor con límite máximo 50.

## Verify
- `check_endpoints.sh` OK.
