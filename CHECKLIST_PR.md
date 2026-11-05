# CHECKLIST_PR.md — Obligatorio

> Constitución §8. 1 revisor. `main` protegida por acuerdo (el plan de GitHub no permite exigirlo). Fusionar a `main` publica en el entorno de desarrollo (nv-08).

- [ ] Spec en `specs/nv-0X` aprobada y enlazada (sin cambio de contrato sin spec)
- [ ] `go build ./...` y `golangci-lint run` sin errores
- [ ] Unit verdes
- [ ] Integration con bases reales (Testcontainers puerto fijo) verdes
- [ ] Security + Database verdes
- [ ] Contract captura `CONTRACT_CAPTURE=1` + convención `snake_case` + `version` presente
- [ ] Endpoints: `docs/endpoints.md` actualizado
- [ ] Rate limits: `docs/rate-limit-matrix.md` verificado; si cambió `TestPresupuestoRedis_*`, estimación de comandos rehecha
- [ ] ASVS: `docs/asvs-l2-checklist.md` verificado
- [ ] Gate del PR en verde: Semgrep, `gosec`, `govulncheck`, Dockle y Trivy sin hallazgos nuevos; excepciones con motivo y fecha
- [ ] Cobertura ≥45% (`scripts/check_coverage.sh --min 45`)
- [ ] Headers + `/ready` + `401` verificados manualmente
- [ ] `make infra-up` y `docker build .` sin errores
- [ ] `Memoria.md` actualizada, sin secretos
- [ ] YAML válido (`yaml.safe_load`), `go build ./...` después de la mutación si se usó
- [ ] Revisor: ______  Fecha: ______
