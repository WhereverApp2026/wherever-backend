# NV-37 — Grupos e integrantes

## Contexto
Un grupo reúne hasta 12 personas alrededor de un plan. **Origen**: US-098, US-099, US-237 y hojas J3 y A4 del Excel. **Depende de**: NV-29/30 (planes) y NV-36 (amistades), ya en `main`.

## Requisitos
1. `POST /grupos` crea el grupo de un plan propio (uno por plan; hasta 12 integrantes contando al organizador) y puede invitar en el mismo paso.
2. `POST /grupos/{id}/integrantes` y `POST /planes/{id}/invitaciones` invitan **solo a amigos aceptados** con cuenta activa. La segunda crea el grupo si el plan aún no lo tiene.
3. El integrante confirma o declina (`PATCH /grupos/{id}/integrantes/yo`) y puede salir (`DELETE …/yo`); el organizador puede sacar a alguien (`DELETE …/{usuario_id}`).
4. El organizador ve quién confirmó, quién sigue invitado y quién declinó; los demás integrantes no ven a quien declinó.
5. Los integrantes invitados o confirmados leen el plan con `GET /planes/{id}`; solo el organizador lo edita, lo borra o lo clona.

## Diseño
- **Paquete** `internal/grupos` (modelo, repositorio, service, handler, DTO y ARCO). `planes` no importa `grupos`: recibe un `planes.Members` por constructor (`WithMembers`), que cumple `grupos.Repository`.
- **Colección `grupos`:** `{_id, plan_id, organizador_id, integrantes[{usuario_id, rol (organizador|integrante), estado (invitado|confirmado|declinado), invitado_por?, desde}], ocupados, creado_en, actualizado_en}`. El organizador va en `integrantes` como `organizador`/`confirmado`. `ocupados` cuenta a todos menos a los que declinaron.
- **Tope de 12** en la misma operación atómica: cada invitación es un `findOneAndUpdate` con `ocupados < 12` en el filtro y `$inc` en la actualización; volver a confirmar después de declinar también lo exige. Leer y luego escribir dejaría pasar a dos a la vez.
- **Varias invitaciones** se procesan una por una, después de validar a todas (amigos activos). Si el grupo no alcanza para todas, se responde 409 antes de invitar; si otra petición llena el grupo a la mitad, las que alcanzaron quedan y la respuesta es 409.
- **Roles:** organizador (todo), integrante (lee, responde, sale; invitar o sacar = 403), ajeno o declinado que ya no está (404, sin revelar que existe). Quien declinó sigue viendo el grupo y puede confirmar mientras haya cupo.
- **Plan borrado:** el grupo deja de verse (404 y fuera de la lista); `GET /planes/{id}` ya no responde a nadie.
- **Re-invitar** a quien declinó lo vuelve a `invitado` (ocupa cupo otra vez). Invitar a quien ya está invitado o confirmado no cambia nada.
- **Lista** `GET /grupos`: los grupos donde soy integrante (cualquier estado), por `actualizado_en` descendente con cursor firmado (nv-17, ámbito: usuario).
- **Límite:** 30 invitaciones por hora por usuario (`rate:grupos_invitacion:{usuario}`) en crear con invitados, `…/integrantes` e `…/invitaciones`.
- **ARCO:** exportar = grupos del usuario con su rol y estado (sin datos de otros); borrar la cuenta = borrar los grupos que organiza y sacarlo de los demás. «Borrar mi historial» no toca grupos.
- **Corrección de drift (2026-11-12):** unirse por enlace (`codigo`, `codigo_vence`, enlace vencido o revocado) es de nv-38; las votaciones y su cierre nocturno son de nv-39; las notificaciones (`invitacion_plan`) se guardan con nv-40 y se envían con nv-41. Por eso esta spec no crea `internal/votaciones`, `internal/planes/compartir.go` ni cambia `privacidad/job.go`.

## Contratos
Filas completas en `docs/endpoints.md`.

| Método | Ruta | Auth | Respuestas |
|---|---|---|---|
| POST | `/api/v1/grupos` | JWT, dueño del plan | 201, 400, 401, 403, 404, 409, 422, 429 |
| GET | `/api/v1/grupos` | JWT | 200, 401, 422 |
| GET | `/api/v1/grupos/{id}` | JWT, integrante | 200, 401, 404 |
| POST | `/api/v1/grupos/{id}/integrantes` | JWT, organizador | 200, 400, 401, 403, 404, 409, 422, 429 |
| POST | `/api/v1/planes/{id}/invitaciones` | JWT, dueño del plan | 200, 400, 401, 403, 404, 409, 422, 429 |
| PATCH | `/api/v1/grupos/{id}/integrantes/yo` | JWT, integrante | 200, 400, 401, 403, 404, 409, 422 |
| DELETE | `/api/v1/grupos/{id}/integrantes/yo` | JWT, integrante | 204, 401, 403, 404, 422 |
| DELETE | `/api/v1/grupos/{id}/integrantes/{usuario_id}` | JWT, organizador | 204, 401, 403, 404, 422 |
| GET | `/api/v1/planes/{id}` | JWT, dueño o integrante | 200, 401, 404 (cambia: antes solo el dueño) |

## Tests
- Unit `internal/grupos/dto_test.go`: validación de cuerpos (ids, máximo 11, sin repetidos, estado), vista del organizador contra la de un integrante.
- Integration `tests/integration/grupos_test.go`: crear con invitados; invitar a quien no es amigo (422) o a cuenta inactiva (422); invitar sin ser organizador (403); ajeno ve 404; confirmar, declinar y volver a confirmar; salir; sacar; el organizador no puede salir ni declinar (422); el integrante lee el plan y no lo edita (404); plan borrado oculta el grupo; lista con cursor; `POST /planes/{id}/invitaciones` crea el grupo una sola vez.
- Concurrencia: 20 invitaciones simultáneas a un grupo de 11 → entra exactamente 1 y `ocupados` = 12.
- Security `tests/security/grupos_security_test.go`: sin token 401; el grupo de otro es 404 y no revela integrantes.
- Database: índices `ux_plan` e `ix_integrante_actualizado`.
- Contract: la exportación ARCO incluye `grupos`.

## Criterios
- Nunca hay más de 12 integrantes que no declinaron.
- Solo amigos aceptados con cuenta activa entran a un grupo.

## Límites
- Sin chat, WebSocket, punto de encuentro, modo sorpresa ni Vaca.
- Sin enlace de invitación (nv-38), votaciones (nv-39) ni notificaciones (nv-40/41).

## Aprobación y Control de Cambios
- **Estado:** 🚧 Borrador revisado (2026-11-12) con la corrección de drift; en ejecución en la rama `nv-37`.
- **Revisores:** Osmar Reséndiz (pendiente de aprobar)
- **Fecha:** —
- **Decisiones para el revisor:** un grupo por plan; el organizador cuenta en los 12; quien declina libera cupo y sigue viendo el grupo; los integrantes no ven a quien declinó; 30 invitaciones por hora.
- **Detalle:** la evidencia irá en `specs/nv-37/checklists/gates.md` + `Memoria.md`.
