# task.md — NV-55 Alta de negocio por pasos y revisión del administrador

## Fase 0 — Preparación
- [ ] T01 Decisiones D1–D11 resueltas; nv-44 y nv-45 en `main`; rama contra `main`.
- [ ] T02 Aviso de privacidad con INE, comprobante, fiscales y CLABE (finalidades, 90 días, 5 años) y `AVISO_VERSION` nueva; tipo de consentimiento `datos_negocio`.
- [ ] T03 Colecciones `negocios_documentos` y `accesos_expediente` con `ux_documento`, `ix_borrar`, `ix_acceso_negocio` (TTL) en `scripts/indexes` y `/ready`.

## Fase 1 — Implementación
- [ ] T04 POST /negocios (borrador, rol `comerciante`, tope de 3) y GET /negocios/mios.
- [ ] T05 PATCH /negocios/{id}/alta/{paso} pasos 2–6 con `If-Match`.
- [ ] T06 Fotos: POST, DELETE y PUT orden sobre la subida de nv-45.
- [ ] T07 Expediente: documentos cifrados en `privado/`, EXIF quitado, tipo real del archivo; GET del dueño.
- [ ] T08 PUT datos-fiscales (RFC) y PUT cuenta-deposito (CLABE) cifrados; consentimiento exigido; política «Expediente del negocio».
- [ ] T09 POST enviar-revision con lista de faltantes.
- [ ] T10 Admin: cola por cursor, lectura con motivo y bitácora fail-closed, decisión con notificación e invalidación de caché.
- [ ] T11 Retención (90 días INE y comprobante, 5 años fiscales) en el trabajo nocturno; Purger y Exporter con bloqueo de fiscales.

## Fase 2 — Cierre
- [ ] T12 Pruebas, `docs/endpoints.md` (filas a la tabla viva), `docs/rate-limit-matrix.md` y `Memoria.md`.
- [ ] T13 `go build`, `go vet`, Unit, lint, `critic`, `check_sdd`, `check_endpoints` en local; Integration, Security, Database y Contract en el CI del PR; evidencia en `gates.md`.
