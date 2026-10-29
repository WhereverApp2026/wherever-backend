---
name: 09-ci-matrix
description: Matriz de CI
---
# 09-ci-matrix

> Rector: `.specify/memory/constitution.md`. Ante conflicto, manda la constitución.

## What I do
- `ci-pr.yml`: deps → build → unit → integration → security → database → contract → critic → endpoints → sdd → coverage → delta-check → mutation → semgrep.
- YAML validado con `yaml.safe_load`.

## Verify
- El YAML parsea y el orden coincide con la constitución §8.
