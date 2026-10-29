---
name: ci-artifacts-tolerant
description: Artifacts robustos
---
# ci-artifacts-tolerant

> Rector: `.specify/memory/constitution.md`. Ante conflicto, manda la constitución.

## What I do
- `if: always()` + `continue-on-error` + `if-no-files-found: warn`.

## Verify
- El CI no falla por artifacts ausentes.
