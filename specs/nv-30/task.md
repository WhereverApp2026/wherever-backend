# task.md — NV-30 Guardar, leer, editar y repetir planes

## Fase 0 — Preparación
- [x] T01 Spec revisada contra nv-29 (drift corregido: 409/400, 404 en plan ajeno, reordenar por horas); rama `nv-30` desde `origin/nv-29`.

## Fase 1 — Implementación
- [x] T02 `POST /planes` con DTO validado y `Calculadora.Aplicar` (`internal/planes/dto.go`, `service.go`, `handler.go`).
- [x] T03 Editar plan (`PATCH`) y paradas: agregar, editar (mover, duración, precio, nota), reordenar y quitar; todas con `If-Match`.
- [x] T04 `GET /planes?pestana=` con `proximos`, `guardados` y `pasados`, cursor por `fecha` e id (`Repository.List`).
- [x] T05 `POST /planes/{id}/clonar`: ids de parada nuevos y mismas horas en el día nuevo.
- [x] T06 Borrado lógico con `If-Match` (`Repository.DeleteIfVersion`); solo el dueño tiene acceso.

## Fase 2 — Cierre
- [x] T07 Pruebas: `internal/planes/dto_test.go` (Unit), `tests/integration/planes_api_test.go`, `tests/security/planes_security_test.go`; `docs/endpoints.md`, `scripts/routes`, `authtest`, `cmd/api` y `Memoria.md`.
- [ ] T08 Local: build, vet, Unit, `golangci-lint`, `critic`, `check_sdd`, `check_endpoints` en verde. Falta el CI del PR (Integration, Security, Database, Contract, cobertura, Semgrep).
