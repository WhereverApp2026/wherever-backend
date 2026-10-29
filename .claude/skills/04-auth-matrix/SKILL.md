---
name: 04-auth-matrix
description: Matriz de autorización
---
# 04-auth-matrix

> Rector: `.specify/memory/constitution.md`. Ante conflicto, manda la constitución.

## What I do
- Cada ruta con su política (público, usuario, rol, dueño) en `docs/endpoints.md`.
- 401 sin token, 403 sin rol, 404 si el recurso no es del usuario.

## Verify
- `check_endpoints.sh` OK: toda ruta tiene fila.
