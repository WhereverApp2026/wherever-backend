# CHECKLIST_PR.md — Obligatorio

> Constitución §8. 1 revisor. `main` protegida.

- [ ] Spec en `specs/nv-0X` aprobada y enlazada (sin cambio de contrato sin spec)
- [ ] `go build ./...` y `golangci-lint run` sin errores
- [ ] Unit verdes
- [ ] Integration con bases reales (Testcontainers puerto fijo) verdes
- [ ] Security + Database verdes
- [ ] Contract captura `CONTRACT_CAPTURE=1` + convención `snake_case` + `version` presente
- [ ] Endpoints: `docs/endpoints.md` actualizado
- [ ] Rate limits: `docs/rate-limit-matrix.md` verificado
- [ ] ASVS: `docs/asvs-l2-checklist.md` verificado
- [ ] Semgrep sin altos; `gosec` y `govulncheck` sin hallazgos nuevos
- [ ] Cobertura ≥45% (`scripts/check_coverage.sh --min 45`)
- [ ] Headers + `/ready` + `401` verificados manualmente
- [ ] `make infra-up` y `docker build .` sin errores
- [ ] `Memoria.md` actualizada, sin secretos
- [ ] YAML válido (`yaml.safe_load`), `go build ./...` después de la mutación si se usó
- [ ] Revisor: ______  Fecha: ______
