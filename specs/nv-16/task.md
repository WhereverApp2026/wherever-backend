# task.md — NV-16 Versión de la API y versión mínima de la app

## Fase 0 — Preparación
- [ ] T01 Aprobar la spec; rama `nv-16`.

## Fase 1 — Implementación
- [ ] T02 Versionado: todas las rutas viven bajo /api/v1.
- [ ] T03 Configuración pública: GET /api/v1/configuracion devuelve version_minima_app,…
- [ ] T04 Versión mínima: la app manda X-App-Version (semver).

## Fase 2 — Cierre
- [ ] T05 Pruebas, documentos y `Memoria.md`.
- [ ] T06 `go build`, `go vet`, cinco suites, `critic`, `check_sdd`, `check_endpoints`; PR; evidencia en `gates.md`.
