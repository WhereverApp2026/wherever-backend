# task.md — NV-32 Itinerario con IA y verificación contra el catálogo

## Fase 0 — Preparación
- [x] T01 Rama `nv-32` desde `origin/nv-31`; drift corregido en la spec (flujo, motivos, colección, pruebas). Falta la firma.

## Fase 1 — Implementación
- [x] T02 Itinerario (US-042): `GenerarItinerario` y `prompts/itinerario.txt`; 2 a 4 paradas con hora y estancia (`internal/asistente/itinerario.go`).
- [x] T03 Verificación (US-043): `Verificar` descarta, reemplaza con el motor y devuelve los descartes; `ia_descartes` con TTL de 30 días (`armar.go`, `descartes.go`, `internal/indexes`).
- [x] T04 Cada parada trae su `por_que` (nv-23), desde el candidato verificado.

## Fase 2 — Cierre
- [x] T05 Pruebas (`verificar_test.go`, `tests/integration/asistente_test.go`, `tests/database/indexes_test.go`), `docs/endpoints.md` y `Memoria.md`.
- [x] T06 Local: `go build`, `go vet`, Unit y `golangci-lint` verdes; `critic`, `check_sdd`, `check_endpoints`. Integration, Security, Database y Contract en el CI del PR.
