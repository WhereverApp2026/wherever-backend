---
name: 07-static-analysis
description: Análisis estático
---
# 07-static-analysis

> Rector: `.specify/memory/constitution.md`. Ante conflicto, manda la constitución.

## What I do
- `golangci-lint` (gosec, errcheck, staticcheck, gocyclo ≤10), Semgrep con `.semgrep/semgrep.yaml`, `govulncheck`.

## Verify
- `golangci-lint run` y `semgrep scan --error` sin hallazgos nuevos.
