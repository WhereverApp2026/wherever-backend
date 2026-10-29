---
name: drift-guards
description: Canónicos y extractores anti-drift
---
# drift-guards

> Rector: `.specify/memory/constitution.md`. Ante conflicto, manda la constitución.

## What I do
- Canónicos: `endpoints.md`, `rate-limit-matrix.md`, `asvs-l2-checklist.md`. `check_endpoints.sh` bloqueante.
- Los extractores normalizan `{id}` igual en código y en docs.

## Verify
- `scripts/check_endpoints.sh` → OK en CI-PR.
