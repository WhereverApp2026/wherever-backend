# task.md — NV-55 Alta de negocio por pasos y revisión del administrador

## Fase 0 — Preparación
- [x] T01 D1–D11 aplicadas como se proponen (firma pendiente); rama contra `main`. nv-45 aún no está en `main`: fotos y documentos detrás de interfaces (503 hasta conectarlas).
- [~] T02 Consentimiento `datos_negocio` hecho (campo en `POST /usuarios/yo/consentimientos`). Pendiente: el aviso con estos datos y `AVISO_VERSION` nueva (rama `aviso-privacidad-integral`) antes de desplegar.
- [x] T03 Colecciones `negocios_documentos` y `accesos_expediente` con `ux_documento`, `ix_borrar`, `ix_acceso_negocio` (TTL) en `scripts/indexes` y `/ready`.

## Fase 1 — Implementación
- [x] T04 POST /negocios (borrador, rol `comerciante`, tope de 3) y GET /negocios/mios.
- [x] T05 PATCH /negocios/{id}/alta/{paso} pasos 2–6 con `If-Match`.
- [~] T06 Fotos: POST, DELETE y PUT orden sobre `alta.Imagenes`; falta conectarla con nv-45 en `cmd/api`.
- [~] T07 Expediente: documentos cifrados en `privado/`, EXIF quitado, tipo real del archivo; GET del dueño. Falta conectar el almacén S3 de nv-45 en `cmd/api`.
- [x] T08 PUT datos-fiscales (RFC) y PUT cuenta-deposito (CLABE) cifrados; consentimiento exigido; política «Expediente del negocio».
- [x] T09 POST enviar-revision con lista de faltantes.
- [x] T10 Admin: cola por cursor, lectura con motivo y bitácora fail-closed, decisión con notificación e invalidación de caché.
- [x] T11 Retención (90 días INE y comprobante, 5 años fiscales) en el trabajo nocturno; Purger y Exporter con bloqueo de fiscales.

## Fase 2 — Cierre
- [x] T12 Pruebas, `docs/endpoints.md` (filas a la tabla viva), `docs/rate-limit-matrix.md` y `Memoria.md`.
- [~] T13 `go build`, `go vet`, Unit, lint, `critic`, `check_sdd`, `check_endpoints` en local; Integration, Security, Database y Contract en el CI del PR; evidencia en `gates.md`.
