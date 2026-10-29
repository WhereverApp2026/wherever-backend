---
name: empirical-verification
description: Medir antes de fijar
---
# empirical-verification

> Rector: `.specify/memory/constitution.md`. Ante conflicto, manda la constitución.

## What I do
- No fijar timeouts, umbrales ni nombres sin medir (`curl /health`, build, test).
- Scripts atómicos (`mktemp` + `mv`), `LC_NUMERIC=C`.

## Verify
- Toda cifra con evidencia de medición citada.
