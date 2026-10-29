---
name: traceability
description: Enlazar spec↔código↔tests
---
# traceability

> Rector: `.specify/memory/constitution.md`. Ante conflicto, manda la constitución.

## What I do
- Cada PR cita spec, archivos, tests y criterios.
- Las specs enlazan `docs/endpoints.md`, no duplican tablas vivas.

## Verify
- `OK` o drift `spec.md:línea ↔ handler.go:línea ↔ test.go:línea`.
