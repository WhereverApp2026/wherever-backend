# Checklist — nv-00 Constitución base

> Trazabilidad: `spec R1-R4 ↔ plan §3-§5 ↔ task T1`. Estado: Borrador con evidencia (2026-11-02), sin firma.

- [x] `specs/nv-00/spec.md` existe con Contexto/Requisitos/Diseño/Contratos/Tests/Criterios/Límites/Aprobación
- [x] Stack fijado (R1): sin Stripe/FCM/OTel/AWS/hosting — `go.mod` solo con `testify`
- [x] Gobernanza SDD + DoD + trazabilidad + 1 revisor (R2) — constitución §11, `AGENTS.md §1-2`, `CHECKLIST_PR.md`
- [x] NFR con comando (R3): `go build ./...` (OK); cobertura `scripts/check_coverage.sh --min 45` documentada, el script llega en `nv-03`
- [x] Reglas no negociables + real-first + sin globals (R4) — constitución §2 y `AGENTS.md §3`
- [x] `AGENTS.md §1` cita el stack + `Memoria.md` existe
- [x] `configs/.env.example` solo con claves usadas, sin secretos (lo verifica `ConfigTest`)
- [x] `.golangci.yml`: gosec + errcheck + staticcheck + gocyclo ≤10 — `golangci-lint@v2.15.0 config verify` OK y `run ./...` → 0 issues
- [x] `ConfigTest (3)` verdes — `go test ./internal/platform/config -count=1` → 3/3
- [x] `critic-guardrails.sh → PASS` en vacío (B1/B2 activos; B3/B4 en SKIP hasta `nv-01`)
- [ ] Aprobación firmada: (pendiente de revisor)

## Evidencia (2026-11-02, Go 1.27.0, Windows + Git Bash)
- `go build ./...` y `go vet ./...` → sin errores.
- `go test ./... -count=1` → `ok .../internal/platform/config`.
- Pruebas negativas manuales: un `.env.example` con `APP_ENV=local`, `STRIPE_KEY`, URI con credenciales y cadena de alta entropía hace fallar los 3 tests; un `.go` con URI con contraseña, `jwtKey` literal y `slog.Info(..., "token", ...)` hace fallar B1 y B2 (exit 1). Tras restaurar, todo vuelve a verde.
