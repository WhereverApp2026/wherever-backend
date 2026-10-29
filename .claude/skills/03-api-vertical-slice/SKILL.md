---
name: 03-api-vertical-slice
description: Molde DTO → validator → service → handler
---
# 03-api-vertical-slice

> Rector: `.specify/memory/constitution.md`. Ante conflicto, manda la constitución.

## What I do
- Cada módulo: `dto.go`, `validator.go`, `service.go`, `repository.go`, `handler.go`, `routes.go`.
- Validators registrados por una sola función; el service recibe sus dependencias por constructor.

## Verify
- Test de arquitectura (`nv-02`) en verde.
