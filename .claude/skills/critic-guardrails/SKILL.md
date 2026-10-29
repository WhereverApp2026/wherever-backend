---
name: critic-guardrails
description: Gate pre-push con 4 bloqueantes
---
# critic-guardrails

> Rector: `.specify/memory/constitution.md`. Ante conflicto, manda la constitución.

## What I do
- `B1` secretos · `B2` token en logs · `B3` pipeline alterado · `B4` ruta sin fila en `endpoints.md` → FAIL.
- `scripts/critic-guardrails.sh` termina en exit 0 si pasa. En CI solo el critic ligero.

## Verify
- `scripts/critic-guardrails.sh` → PASS.
