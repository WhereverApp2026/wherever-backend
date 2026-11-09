# task.md — NV-49 Cifrar los datos sensibles en la base

## Fase 0 — Preparación
- [ ] T01 Aprobar la spec; rama `nv-49`.

## Fase 1 — Implementación
- [ ] T02 Lectura y escritura transparentes para el resto del código mediante una capa en el…
- [ ] T03 Migración idempotente de los datos que ya estén en claro.
- [ ] T04 El exportador de nv-05 entrega los datos descifrados al dueño; el purgador los borra.
- [ ] T05 Nada sensible en claro en logs, errores ni copias de seguridad de pruebas.

## Fase 2 — Cierre
- [ ] T06 Pruebas, documentos y `Memoria.md`.
- [ ] T07 `go build`, `go vet`, cinco suites, `critic`, `check_sdd`, `check_endpoints`; PR; evidencia en `gates.md`.
