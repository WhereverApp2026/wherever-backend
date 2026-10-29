---
name: 10-testing-strategy
description: Qué suite cubre qué (lite)
---
# 10-testing-strategy

> Rector: `.specify/memory/constitution.md`. Ante conflicto, manda la constitución.

## What I do
- Unit lógica; Integration y Security con la app real y bases reales; Database índices; Contract captura.
- Puerta incremental advisory `nv-03`.
- Sin rendimiento ni caos.

## Verify
- `go test ./... -count=1` por suite + cobertura ≥45%.
