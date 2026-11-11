# task.md — NV-24 Recomendaciones y «No me interesa»

## Fase 0 — Preparación
- [x] T01 Rama `nv-24` sobre `origin/nv-23` (usa su recomendador); drift corregido en la spec. Falta la firma.

## Fase 1 — Implementación
- [x] T02 `GET /recomendaciones` devuelve «Para ti ahora» con su porqué; acepta compañía (solo, pareja, amigos, familia) y hora.
- [x] T03 `POST` y `DELETE /recomendaciones/{negocio_id}/no-me-interesa` sobre `descartes_usuario` (índice único) y ARCO.
- [x] T04 Aprendizaje apagado: sin gustos ni presupuesto guardados; alergias solo con consentimiento y solo como filtro duro.

## Fase 2 — Cierre
- [x] T05 Pruebas (Unit, Integration, Security, Database), `docs/endpoints.md` y `Memoria.md`.
- [ ] T06 `go build`, `go vet`, Unit, lint, `critic`, `check_sdd`, `check_endpoints` en local ✅; Integration, Security, Database y Contract en el CI del PR; evidencia en `gates.md`.
