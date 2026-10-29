---
name: 01-startup-health
description: Salud: /health y /ready con checks reales
---
# 01-startup-health

> Rector: `.specify/memory/constitution.md`. Ante conflicto, manda la constitución.

## What I do
- `/health` siempre 200 `{estado:"ok", version}` sin tocar bases. `/ready` consulta MongoDB, Redis y Neo4j con 1 s máximo cada una y responde `{estado, dependencias}`: 503 si cae MongoDB o Neo4j; Redis caído = 200 `degraded`.
- Sin datos sensibles en la respuesta.

## Verify
- `curl /health` 200; `curl /ready` 200/503 según las bases.
