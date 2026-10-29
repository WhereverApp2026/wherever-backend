# Checklist Template — Gates PR

> Orden CI-PR: deps → build → unit → integration → security → database → contract → critic → endpoints → sdd → coverage → delta-check → mutation → semgrep.

- [ ] `go build ./...` y `go vet` sin errores; `golangci-lint` sin errores
- [ ] Unit + Integration con bases reales (Testcontainers puerto fijo) verdes
- [ ] Security + critic + semgrep sin hallazgos altos
- [ ] `docs/endpoints.md` actualizado y enlazado desde la spec
- [ ] Contract captura `CONTRACT_CAPTURE=1` + convención `snake_case`
- [ ] Cobertura ≥45% (`scripts/check_coverage.sh --min 45`)
- [ ] Puerta incremental advisory: delta ≥80% + mutación enfocada sin break (no bloquea)
- [ ] Headers verificados (nosniff, DENY, CORS 1 origen, HSTS no-dev, CSP)
- [ ] `Memoria.md` actualizada, sin secretos
- [ ] 1 revisor, `main` protegida
