# task.md — NV-39 Votaciones

## Fase 0 — Preparación
- [ ] T01 Aprobar la spec (firma pendiente); rama `nv-39` desde `nv-38`.

## Fase 1 — Implementación
- [x] T02 Abrir una votación de 2 o 3 opciones con hora límite; una abierta por parada.
- [x] T03 Un voto por persona, editable hasta el cierre; invitados y confirmados.
- [x] T04 Cerrar a mano o al vencer (trabajador por minuto sin Redis), una sola vez con dos instancias; empate = la propuesta primero.
- [x] T05 Aplicar la ganadora al plan y dejar constancia (`aplicada`, `motivo`).

## Fase 2 — Cierre
- [x] T06 Pruebas, documentos y `Memoria.md`.
- [ ] T07 `go build`, `go vet`, cinco suites, `critic`, `check_sdd`, `check_endpoints`; PR; evidencia en `gates.md`.
