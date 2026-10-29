---
name: deferred-scope-fakes
description: Real-first con NOTEs, sin fakes
---
# deferred-scope-fakes

> Rector: `.specify/memory/constitution.md`. Ante conflicto, manda la constitución.

## What I do
- Prohibido hash falso, token opaco o base simulada. Solo `NOTE(real-first)` + test que aserta lo diferido (sin caché de entidades, sin OTel completo).

## Verify
- `grep` 0 fakes + tests asertan los diferidos.
