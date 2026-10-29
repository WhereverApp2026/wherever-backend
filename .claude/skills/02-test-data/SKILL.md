---
name: 02-test-data
description: Datos y helpers de prueba
---
# 02-test-data

> Rector: `.specify/memory/constitution.md`. Ante conflicto, manda la constitución.

## What I do
- `internal/testutil`: fábrica de datos, generador de tokens (válido, vencido, sin firma, `alg:none`), arranque de contenedores con puerto fijo.
- Datos aislados por prueba (colección limpia).

## Verify
- Las suites Security e Integration reutilizan `testutil`.
