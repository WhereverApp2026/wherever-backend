# task.md — NV-30 Guardar, leer, editar y repetir planes

## Fase 0 — Preparación
- [ ] T01 Aprobar la spec; rama `nv-30`.

## Fase 1 — Implementación
- [ ] T02 POST /planes guarda un plan (generado o hecho a mano) con nombre, fecha, personas,…
- [ ] T03 Editar: cambiar nombre, fecha, personas, presupuesto o movilidad; agregar, mover, cambiar…
- [ ] T04 GET /planes?pestana= lista por pestaña (próximos, guardados, pasados; grupales llega…
- [ ] T05 Repetir (US-058): POST /planes/{id}/clonar duplica el plan propio con una fecha…
- [ ] T06 Borrado lógico del plan; un plan con integrantes de grupo solo lo borra el dueño.

## Fase 2 — Cierre
- [ ] T07 Pruebas, documentos y `Memoria.md`.
- [ ] T08 `go build`, `go vet`, cinco suites, `critic`, `check_sdd`, `check_endpoints`; PR; evidencia en `gates.md`.
