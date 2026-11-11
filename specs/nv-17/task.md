# task.md — NV-17 Paginación por cursor y validadores HTTP

## Fase 0 — Preparación
- [ ] T01 Aprobar las seis decisiones de la spec; rama `nv-17` desde `main`.

## Fase 1 — Implementación
- [ ] T02 `internal/shared/pagination`: `Signer` HMAC-SHA-256, `Cursor{ID,Sort,Scope}` con `ambito`, `Encode/Verify/Decode`, `Valid` para la forma y `ErrInvalidCursor`; pruebas de ida y vuelta, manipulación, ámbito ajeno, llave corta y fuzz.
- [ ] T03 Validación: `validCursor` usa `pagination.Valid`; `PaginationQuery.Position(signer, scope)` verifica firma y ámbito y devuelve 422 `cursor`; pruebas de límites y ámbito ajeno.
- [ ] T04 ETag en `GET /ciudades/autocompletar`: serializar una vez, `ETag` débil, `Cache-Control: public, max-age=60` y 304 con `If-None-Match`; pruebas de integración.
- [ ] T05 `Cache-Control: no-store` en `GET /usuarios/yo` y `GET /auth/sesiones`; comprobar que no hay 304.
- [ ] T06 OpenAPI y `docs/endpoints.md` (filas y notas de paginación/ETag), `Memoria.md`.

## Fase 2 — Cierre
- [ ] T07 `go build`, `go vet`, cinco suites `-count=1`, cobertura ≥ 45 %, Semgrep, `critic`, `check_endpoints` y `check_sdd`; evidencia en `gates.md`.
- [ ] T08 Servidor: desplegar y comprobar por CloudFront el 200/304 de autocompletar y el `no-store` de las privadas. En este corte no se crea `CURSOR_HMAC_KEY`; la primera lista posterior la agrega antes de desplegarse.
