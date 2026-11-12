# task.md — NV-17 Paginación por cursor y validadores HTTP

## Fase 0 — Preparación
- [x] T01 Aprobar las seis decisiones de la spec (2026-11-12, con el ajuste de migrar `/lugares` y `/planes`); rama `claude/nv-17-af35a1` desde `main`.

## Fase 1 — Implementación
- [x] T02 `internal/shared/pagination`: `Signer` HMAC-SHA-256, `Cursor{ID,Sort,Scope}` con `ambito`, `Encode/Decode/Scope`, `Valid` para la forma y `ErrInvalidCursor`; pruebas de ida y vuelta, manipulación, ámbito ajeno, llave corta y fuzz.
- [x] T03 Validación: `validCursor` usa `pagination.Valid`; `PaginationQuery.Position(signer, scope)` verifica firma y ámbito y devuelve 422 `cursor`; pruebas de límites y ámbito ajeno.
- [x] T04 ETag en `GET /ciudades/autocompletar`: serializar una vez, `ETag` débil, `Cache-Control: public, max-age=60` y 304 con `If-None-Match`; pruebas de integración.
- [x] T05 `Cache-Control: no-store` en `GET /usuarios/yo` y `GET /auth/sesiones`; comprobar que no hay 304.
- [x] T05b Migrar `GET /lugares` y `GET /planes` al cursor firmado con ámbito; `CURSOR_HMAC_KEY` en config, `.env.example`, `deploy-env.sh`, `nightly.yml` y `testutil`.
- [x] T06 OpenAPI y `docs/endpoints.md` (filas y notas de paginación/ETag), `Memoria.md`.

## Fase 2 — Cierre
- [ ] T07 (local verde: build, vet, Unit, lint, critic, sdd, endpoints; Integration/Security/Database/Contract en el CI del PR) `go build`, `go vet`, cinco suites `-count=1`, cobertura ≥ 45 %, Semgrep, `critic`, `check_endpoints` y `check_sdd`; evidencia en `gates.md`.
- [ ] T08 Servidor: desplegar y comprobar por CloudFront el 200/304 de autocompletar y el `no-store` de las privadas. Antes, el operador carga `/wherever/dev/CURSOR_HMAC_KEY` (SecureString, 32+ bytes).
