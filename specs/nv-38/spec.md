# NV-38 — Enlace de invitación y vista pública

## Contexto
Quien no tiene la app puede ver el plan y entrar con un enlace seguro. **Origen**: US-098, US-099, US-237 y hojas J3 y A4 del Excel. **Depende de**: NV-37.

## Requisitos
1. **Enlace:** `POST /planes/{id}/compartir` crea un código aleatorio de 128 bits que vence en 7 días; el organizador lo revoca con `DELETE` o lo reemplaza creando otro.
2. **Vista pública:** con el código, cualquiera ve el plan (nombre, fecha, paradas con sus lugares, costo por persona y cupo) sin sesión y sin ver a ninguna persona.
3. **Unirse:** con sesión, el código deja a la persona confirmada en el grupo, sin pedir amistad; respeta el tope de 12 y los bloqueos.

## Diseño
- **Código:** 16 bytes de `crypto/rand` en base64url (22 caracteres). En `grupos` se guarda solo `codigo_hash` (SHA-256, índice único parcial `ux_codigo_hash`) y `codigo_vence`. Un código por grupo: crear otro reemplaza al anterior, que deja de servir. Compartir crea el grupo si el plan no lo tiene.
- **El código nunca va en una ruta:** la ruta de cada petición queda en el log de acceso (y en los de CloudFront). Por eso la vista pública y unirse reciben el código en el cuerpo (`POST {codigo}`), y el enlace lleva el código en el fragmento: `{APP_LINK_BASE}/plan#codigo=…`. Es la misma regla de los enlaces de correo de nv-06; la página o la app lo leen de `location.hash` (o del intent del App Link).
- **Sin compresión** en `POST /planes/{id}/compartir`, porque responde el código (`carriesTokens`, nv-12).
- **Vista pública:** plan activo con la tarjeta de `GET /lugares` de cada negocio visible (`negocios.Repository.FindCards`); sin ids de cuentas, organizador, integrantes, notas, presupuesto ni origen. Inexistente, revocado, vencido o de un plan borrado = el mismo 404.
- **Unirse:** entra confirmado; si estaba invitado o había declinado, pasa a confirmado (volver exige cupo). Es una sola operación con `ocupados < 12`. Vencido = 410; bloqueo entre organizador y quien entra = 404; sin cupo = 409; el organizador recibe su grupo sin cambios.
- **Límites:** vista pública 30 por minuto por IP (`rate:plan_compartido:{ip}`); unirse 30 por hora por usuario (`rate:grupos_unirse:{usuario}`). Con 128 bits no frenan adivinanzas, frenan el escaneo.
- **Corrección de drift (2026-11-12):** la spec original ponía el código en la ruta (`GET /planes/compartidos/{codigo}` y `POST /grupos/unirse/{codigo}`) y no tenía cómo revocar. Ahora va en el cuerpo y existe `DELETE /planes/{id}/compartir`.

## Contratos
Filas completas en `docs/endpoints.md`.

| Método | Ruta | Auth | Respuestas |
|---|---|---|---|
| POST | `/api/v1/planes/{id}/compartir` | JWT, dueño del plan | 201, 401, 403, 404 |
| DELETE | `/api/v1/planes/{id}/compartir` | JWT, dueño del plan | 204, 401, 403, 404 |
| POST | `/api/v1/planes/compartidos` con `{codigo}` | no | 200, 400, 404, 422, 429 |
| POST | `/api/v1/grupos/unirse` con `{codigo}` | JWT | 200, 400, 401, 403, 404, 409, 410, 422, 429 |

## Tests
- Unit `internal/grupos/dto_test.go`: el código tiene 22 caracteres, es distinto cada vez y solo se guarda su hash; la vista pública no lleva personas, notas ni presupuesto.
- Unit `internal/http/middleware/compress_test.go`: `POST /planes/{id}/compartir` no se comprime.
- Integration `tests/integration/enlace_test.go`: compartir, ver sin sesión y unirse; invitado o declinado quedan confirmados; reemplazado, vencido (404 en la vista, 410 al unirse) y revocado; plan ajeno, bloqueo y plan borrado; grupo lleno = 409.
- Security `tests/security/enlace_security_test.go`: el código no se guarda en claro ni llega a los logs; la vista pública no revela correo, organizador ni notas; límite por IP; compartir, revocar y unirse piden sesión.
- Database: `ux_codigo_hash` único.

## Criterios
- La vista pública no revela personas.
- El código no se guarda ni se registra en claro.

## Límites
- La página web la pone la app o un sitio estático: la API solo sirve datos.
- Los Android App Links los configura la app. Ojo: la URL de CloudFront cambia al recrear la infraestructura (nv-14), y con ella `APP_LINK_BASE`.

## Aprobación y Control de Cambios
- **Estado:** 🚧 Borrador revisado (2026-11-12) con la corrección de drift; en ejecución en la rama `nv-38`.
- **Revisores:** Osmar Reséndiz (pendiente de aprobar)
- **Fecha:** —
- **Decisiones para el revisor:** el código va en el cuerpo y en el fragmento (no en la ruta); un enlace por grupo, que se reemplaza al crear otro; unirse no pide amistad; vencido = 404 en la vista y 410 al unirse.
- **Detalle:** la evidencia irá en `specs/nv-38/checklists/gates.md` + `Memoria.md`.
