# task.md — NV-28 Límite por usuario y protección de datos en la IA

## Fase 0 — Preparación
- [ ] T01 Aprobar la spec; rama `nv-28`.

## Fase 1 — Implementación
- [ ] T02 Límite por usuario y por hora (IA_LIMITE_HORA, 10 por defecto) con Retry-After y…
- [ ] T03 Defensa contra inyección: el texto del usuario va solo en el turno de usuario, nunca…
- [ ] T04 Sin datos personales hacia el modelo: sin nombre, correo, teléfono ni coordenadas…

## Fase 2 — Cierre
- [ ] T05 Pruebas, documentos y `Memoria.md`.
- [ ] T06 `go build`, `go vet`, cinco suites, `critic`, `check_sdd`, `check_endpoints`; PR; evidencia en `gates.md`.
