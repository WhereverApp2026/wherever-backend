# task.md — NV-24 Recomendaciones y «No me interesa»

## Fase 0 — Preparación
- [ ] T01 Aprobar la spec; rama `nv-24`.

## Fase 1 — Implementación
- [ ] T02 GET /recomendaciones devuelve «Para ti ahora»; acepta compañía (solo, pareja, amigos,…
- [ ] T03 POST y DELETE /recomendaciones/{negocio_id}/no-me-interesa.
- [ ] T04 Si el usuario apagó el aprendizaje personalizado o no dio consentimiento de datos…

## Fase 2 — Cierre
- [ ] T05 Pruebas, documentos y `Memoria.md`.
- [ ] T06 `go build`, `go vet`, cinco suites, `critic`, `check_sdd`, `check_endpoints`; PR; evidencia en `gates.md`.
