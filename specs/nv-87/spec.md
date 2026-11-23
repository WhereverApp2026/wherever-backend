# NV-87 — Mensajes directos

## Contexto
La maqueta tiene **mensajes directos** entre amigos: la sección «Mensajes directos · + Nuevo mensaje» en `44_MisGrupos`, el chat `ChatAmigo` (con «Invitar a un plan» y «Compartir lugar»), `InvitarPlan`, el botón «✉ Mensaje» del perfil (`26_PerfilAmigo`) y «Responde a Jorge…» en las historias (`16_Historia`). Hoy solo existe el chat de grupo (nv-57: `grupos/{id}/mensajes`, reacciones, push agrupado, sin canal en vivo). **Origen**: revisión de la maqueta (2026-11-21). **Depende de**: NV-36 (amistades y bloqueo), NV-57 (chat de grupo), NV-38 (invitación a plan), NV-63 (momentos), NV-40 (avisos), NV-82 (privacidad).

## Requisitos
1. Una conversación privada entre dos amigos, con su lista y conteo de no leídos.
2. Mensajes de texto y adjuntos de plan, lugar y momento (respuesta a historia).
3. Invitar a un plan desde la conversación.
4. Un bloqueo corta la conversación; borrar la cuenta borra lo suyo.

## Diseño
- **Modelo:** colección nueva `conversaciones` `{_id, par (ids ordenados), participantes[2], ultimo_mensaje{texto, autor_id, creado_en}, leido_hasta{usuario_id: ObjectId}, creada_en}` con índice único `ux_conversacion_par`. Los mensajes reutilizan `mensajes` de nv-57 con `conversacion_id` en lugar de `grupo_id` (índice `ix_mensajes_conversacion`), las mismas reacciones, `id_cliente` idempotente y borrado que conserva el lugar.
- **Rutas:**
  - `POST /conversaciones` `{usuario_id}` → la conversación (la existente si ya hay); solo con amistad aceptada y sin bloqueo (si no, 404 igual).
  - `GET /conversaciones` → lista por último mensaje con `no_leidos`, el otro participante (`id, nombre, nombre_usuario, foto_url`) y la vista previa.
  - `GET/POST /conversaciones/{id}/mensajes` (cursor; `POST` con `texto` 1–1000 y adjunto opcional `{tipo: plan|lugar|momento, id}` que el servidor resuelve y verifica que yo pueda ver), `DELETE /conversaciones/{id}/mensajes/{mid}` (solo el autor), `PUT /conversaciones/{id}/mensajes/{mid}/reacciones`, `POST /conversaciones/{id}/leido`.
- **Invitar a un plan:** el adjunto `plan` + `POST /planes/{id}/invitaciones` de nv-38; la tarjeta del mensaje muestra «Invitación enviada» y «se unió al plan» cuando acepta.
- **Moderación:** texto con las reglas de nv-71 (rechazado = 422); sin moderación por IA en mensajes privados (decisión para el revisor).
- **Avisos:** `mensaje_directo` (categoría social, nv-86) agrupado por conversación como el de grupo; respeta silencio (nv-70) y horario silencioso.
- **Sin «En línea»:** no hay canal en vivo (como nv-57); la app actualiza al abrir y con push.
- **Privacidad y ARCO:** bloqueo = la conversación desaparece para los dos y no se puede escribir; borrar la cuenta borra sus mensajes (quedan «Mensaje eliminado») y la conversación si el otro también la borró. Exportación de mis conversaciones y mensajes propios. Logs sin texto.
- **Límite:** `mensajes_directos` 120 por hora por usuario.

## Contratos
| Método | Ruta | Auth | Respuestas |
|---|---|---|---|
| POST | `/api/v1/conversaciones` | JWT, cuenta activa | 200/201, 401, 403, 404, 422 |
| GET | `/api/v1/conversaciones` | JWT, cuenta activa | 200, 401, 403 |
| GET/POST | `/api/v1/conversaciones/{id}/mensajes` | JWT, participante | 200/201, 401, 403, 404, 409, 422, 429 |
| DELETE | `/api/v1/conversaciones/{id}/mensajes/{mid}` | JWT, autor | 204, 401, 404 |
| PUT | `/api/v1/conversaciones/{id}/mensajes/{mid}/reacciones` | JWT, participante | 200, 401, 404, 422 |
| POST | `/api/v1/conversaciones/{id}/leido` | JWT, participante | 204, 401, 404 |

## Tests
- Integration: crear o reutilizar la conversación; mensajes con `id_cliente`; adjuntos verificados; no leídos; invitación a plan; avisos agrupados; bloqueo; ARCO.
- Security: no participante = 404; sin amistad = 404; adjunto de un plan o momento que no puedo ver = 422 sin filtrar datos; texto fuera de los logs; límite 429.
- Database: `ux_conversacion_par`, `ix_mensajes_conversacion`. Contract: las seis rutas.

## Criterios
- Solo los dos participantes leen la conversación y un bloqueo la corta de inmediato.

## Límites
- Sin mensajes a no amigos, sin grupos desde aquí (para eso nv-37), sin canal en vivo, sin audios ni archivos.

## Drift al ejecutar (2026-11-23)
- **Código:** dentro de `internal/chat` (reutiliza repositorio, reacciones, cursor y `CleanText` de nv-57): `directos_model.go`, `directos_repository.go` (`ConversationRepository`, `UnreadCounts`, `Newest`), `directos_dto.go`, `directos_service.go` (`DirectService`), `directos_handler.go` (`DirectHandler`, módulo aparte en `main.go`, `authtest` y `scripts/routes`) y `directos_privacy.go` (`DirectPrivacy`). El repositorio de `mensajes` recibe la conversación (`grupo_id` o `conversacion_id`) en vez del id del grupo; `Message` ganó `conversacion_id` y `adjunto`, y `grupo_id` pasó a `omitempty`.
- **Índices:** además de `ux_conversacion_par` y `ix_mensajes_conversacion`: `ix_conversacion_participante` `{participantes, actualizada_en}` (la lista y ARCO) y `ux_id_cliente_conversacion` `{conversacion_id, id_cliente}` parcial (el `ux_id_cliente` de nv-57 es por grupo). Hay que crearlos en Atlas (`go run ./scripts/indexes`) antes del despliegue. El `ux_id_cliente` viejo también ve los directos con `grupo_id` nulo: dos UUID v4 iguales en conversaciones distintas darían 503; no se cambió para no recrear un índice vivo.
- **Lectura y escritura:** leer, borrar lo propio y marcar leído piden participar sin bloqueo; escribir y reaccionar piden además la amistad aceptada (si la eliminaron, el historial se lee pero no se escribe). Todo lo demás es 404 igual. Todas las rutas piden cuenta activa (403).
- **Lista:** sin paginación, las 100 más recientes (`MaxConversations`); una conversación abierta sin mensajes no sale. `otro` sale `null` si la cuenta ya no está activa. `no_leidos` se cuenta en una sola agregación.
- **Invitar a un plan:** sin ruta nueva. El mensaje lleva el adjunto `plan` y la app llama `POST /planes/{id}/invitaciones`; la tarjeta trae `adjunto.invitacion` = `enviada` (quien lo recibió está `invitado` en el grupo del plan) o `aceptada` (`confirmado`), calculado al leer.
- **Adjuntos:** el servidor guarda `{tipo, id, nombre}` (nombre del plan o del lugar al enviarlo). Plan con `planes.Service.Visible` (nv-83/nv-84, respeta `compartir_planes`), lugar visible en `negocios`, momento con `postales.Service.Visible` (respeta `actividad` y `compartir_postales` de nv-82). Lo que no veo o no existe = 422 `adjunto`.
- **Avisos:** `mensaje_directo` con `notificaciones.Agrupador` (uno por persona y conversación cada 10 min, `datos.conversacion_id`, sin texto ni autor); no se avisa si quien recibe silenció a quien escribe (nv-70); el horario silencioso lo aplica el push. Va en la categoría `social` de nv-86.
- **Límite:** 120 por hora contados en MongoDB (`ix_autor`, solo mensajes con `conversacion_id`), sin política de Redis: no sube `TestPresupuestoRedis_*`. El límite de 30/min del chat de grupo ya no cuenta los directos.
- **ARCO:** `DirectPrivacy` (`conversaciones` en la exportación: conversaciones con el id de la otra persona, mis mensajes y mis reacciones); borrar la cuenta vacía sus mensajes sin autor, quita su lectura y su vista previa, y borra la conversación con sus mensajes cuando la otra persona ya había borrado la suya. El exportador del chat de grupo dejó de incluir los directos.
- **Pruebas:** Unit `internal/chat/directos_test.go`; Integration `tests/integration/directos_test.go`; Security `tests/security/directos_security_test.go`; Contract `tests/contract/directos_contract_test.go` (`testdata/directos/*`) y `usuarios/exportacion_descargar_200` con `conversaciones`; Database en `tests/database/indexes_test.go`.

## Aprobación y Control de Cambios
- **Estado:** 🚧 Borrador con evidencia (2026-11-23), ejecutada; ver `checklists/gates.md`.
- **Revisores:** Osmar Reséndiz (pendiente de aprobar)
- **Fecha:** —
- **Decisiones para el revisor:** (1) solo entre amigos; (2) reutilizar `mensajes` de nv-57 con `conversacion_id`; (3) sin «En línea»; (4) moderación solo por reglas en privado. Se ejecutó con las cuatro como están.
- **Detalle:** `specs/nv-87/checklists/gates.md` al ejecutar.
