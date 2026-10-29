---
name: kill-tree
description: Matar procesos de prueba por nombre
---
# kill-tree

> Rector: `.specify/memory/constitution.md`. Ante conflicto, manda la constitución.

## What I do
- Matar el binario por su nombre (`pkill -f bin/api`), nunca `go`. Limpiar contenedores huérfanos antes de correr suites.

## Verify
- Sin procesos ni contenedores huérfanos tras la suite.
