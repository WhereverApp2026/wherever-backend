# task.md — NV-27 Tope de gasto y caché de la IA

## Fase 0 — Preparación
- [x] T01 Rama `nv-27` desde `main` (solo depende de nv-26). Falta la firma de la spec.

## Fase 1 — Implementación
- [x] T02 Tope de gasto mensual: `ia.Guardia`, `cuota:ia_gasto:{mes}`, `usos_ia`, `IA_TOPE_USD_MES` y `IA_PRECIO_*`.
- [x] T03 Caché `cache:ia:{huella}`, 1 hora, solo salidas validadas y sin la frase.

## Fase 2 — Cierre
- [x] T04 Pruebas, documentos y `Memoria.md`.
- [ ] T05 En local `go build`, `go vet`, Unit, lint, `critic`, `check_sdd` y `check_endpoints` (listo); Integration, Security, Database, Contract y cobertura en el CI del PR (pendiente); evidencia en `gates.md`.
