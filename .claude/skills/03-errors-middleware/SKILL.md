---
name: 03-errors-middleware
description: Errores uniformes
---
# 03-errors-middleware

> Rector: `.specify/memory/constitution.md`. Ante conflicto, manda la constitución.

## What I do
- Un solo `ErrorHandler` con el mapeo de constitución §5: `BadRequest→400, Unauthorized→401, Forbidden→403, NotFound→404, Timeout→408, Conflict→409, Gone→410, Validation→422, Locked→423, RateLimited→429, Unavailable→503, otros→500`. Siempre con `traza_id`.
- Sin `recover` ad-hoc en handlers. Errores tipados en `internal/shared/apperr`.

## Verify
- Tabla de mapeos probada en Unit.
