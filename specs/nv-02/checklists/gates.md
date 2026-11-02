# Checklist — nv-02 Molde

> Trazabilidad: `spec §2-§6 ↔ plan §3-§5 ↔ task T1`. Estado: Borrador con evidencia (2026-11-02), sin firma.

- [x] `PageResult` con `items/siguiente_cursor/limite` y cursor opaco — `TestPageResult_SerializesContractFields`, `cursor_test.go`
- [x] Registro único de validators; `PaginationQuery` válido (1,20) / inválido (0,500) — `TestValidators_RegisteredFromSinglePoint`, `TestPaginationQuery_LimitRange`
- [x] Errores tipados de constitución §5 mapeados (400/401/403/404/408/409/410/422/423/429/503) — `apperr_test.go`; el `ErrorHandler` de nv-01 ya los traduce
- [x] `arch_test (6)` y `cursor_test (4)` verdes, más `registry_test (4)` y `apperr_test (2)`
- [x] Ningún handler nuevo, ninguna colección, `endpoints.md` intacto
- [x] `critic PASS`, `endpoints OK`, `coverage ≥45%` (48,3 %), `semgrep 0`
- [ ] Aprobación firmada: (pendiente de revisor)

## Evidencia (2026-11-02, Go 1.27.0 local, Docker 29.8.1, Windows + Git Bash)
- `go build ./...` y `go vet ./...` sin errores; `golangci-lint@v2.15.0 run ./...` → 0 issues.
- `go test ./internal/... ./cmd/... -count=1` → 46 PASS (30 de nv-01 + 16 nuevos). `./tests/security/...` y `./tests/integration/... -p 1` siguen verdes.
- Cobertura `go test ./internal/... ./cmd/... -coverprofile` → 48,3 % total (`validation` 87,1 %). `scripts/check_coverage.sh` llega en nv-03.
- `critic-guardrails` PASS, `check_endpoints` OK (10 rutas, sin cambio), `check_sdd` SDD-OK (4 specs).
- `semgrep 1.95.0 scan --config=auto --config=.semgrep/semgrep.yaml --error` (imagen Docker) → 0.
- `govulncheck@v1.9.0` → 0 vulnerabilidades en módulos; las 9 restantes son de la librería estándar del Go 1.27.0 local (CI usa 1.27.x).
- Prueba negativa del test de arquitectura: un archivo en `internal/http` con import de `mongo-driver` y `recover()`, y otro con `RegisterValidation` fuera del registro → fallan los 3 tests correspondientes; al quitarlos vuelven a verde.
- `CONTRACT_CAPTURE` no aplica: no hay contrato nuevo.
