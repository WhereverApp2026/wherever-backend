# NV-57 — Chat de grupo

## 1. Objetivo
Chat ligero dentro de un grupo de nv-37: texto, reacciones y avisos del sistema, con historial paginado, búsqueda y un canal en vivo opcional.

- **Origen:** US-104 a US-108 y hoja «J3 Grupos y chat» del Excel (filas de mensajes y `tiempo-real`).
- **Depende de:** NV-37 (grupos e integrantes), NV-40 y NV-41 (aviso por push a quien no está conectado), NV-17 (cursor firmado).
- **Lo usa:** NV-39 (aviso "Ganó … con N votos" al cerrar una votación).

## 2. Alcance / No-alcance
- **Dentro:** colección `mensajes`, campo `grupos.ultimo_mensaje`, las 5 rutas de mensajes, búsqueda `?q=`, canal en vivo (si el revisor lo aprueba, ver §7), `Purger` y `Exporter`, índices y pruebas.
- **Fuera (ya cubierto):** crear, listar, unirse e integrantes (nv-37), enlace (nv-38), votaciones (nv-39).
- **Fuera (sin spec aún):** `PATCH`/`DELETE /grupos/{id}`, punto de encuentro (`/punto-encuentro`, `/sugerido`, `/traslados`), modo sorpresa y `me-apunto`. Los eventos en vivo de votos, gastos y cambios del plan los agrega la spec de cada uno sobre el mismo canal.
- **Fuera:** editar mensajes (el Excel trae `editado_en` pero ninguna ruta), adjuntos, fotos, mensajes de voz, cifrado de extremo a extremo y reportar mensajes.

## 3. Contratos API
Filas de J3 corregidas contra lo que existe. Al programar se copian a `docs/endpoints.md`.

| Método | Ruta | Entrada | Auth | Respuestas |
|---|---|---|---|---|
| GET | `/api/v1/grupos/{id}/mensajes` | `?antes_de=&despues_de=&q=&limite=` | JWT integrante confirmado | 200, 401, 403, 404, 422 |
| POST | `/api/v1/grupos/{id}/mensajes` | `{texto, id_cliente}` | JWT integrante confirmado | 201, 200 (reenvío), 400, 401, 403, 404, 422, 429 |
| PUT | `/api/v1/grupos/{id}/mensajes/{mid}/reacciones` | `{emoji}` (`null` la quita) | JWT integrante confirmado | 200, 400, 401, 403, 404, 422 |
| DELETE | `/api/v1/grupos/{id}/mensajes/{mid}` | — | JWT autor | 204, 401, 403, 404 |
| GET (WS) | `/api/v1/grupos/{id}/tiempo-real` | `Authorization: Bearer` en el saludo | JWT integrante confirmado | 101, 401, 403, 404, 429 |

- **Respuesta de mensaje:** `{id, autor{id, nombre, foto_url}?, tipo, texto, reacciones[{emoji, total, mia}], eliminado, creado_en}`. Un mensaje eliminado sale con `eliminado: true`, sin texto ni reacciones.
- **Lista:** `PageResult` del más nuevo al más viejo; `limite` 30 por defecto y 100 máx.
  - `antes_de` y `despues_de` son ids de mensaje (el `_id` ya ordena por tiempo); `despues_de` sirve para ponerse al día al reconectar.
  - Con `q` (2 a 100 caracteres) busca solo en ese grupo con `ix_chat_texto` y pagina con el cursor firmado de nv-17.
- **403** = cuenta en eliminación pendiente (`usuarios.RequireActive`). **404** = grupo inexistente, borrado o ajeno, o el usuario no es integrante confirmado (no se distingue, igual que nv-37).
- **Drift corregido frente al Excel:**
  - `PUT reacciones` "pone o quita": `emoji: null` la quita; una reacción por persona y mensaje; solo 6 emojis fijos (lista en el DTO) para no guardar texto libre.
  - `editado_en` se quita del modelo: no hay ruta para editar.
  - El tipo `votacion` se cubre con `sistema`.
  - El token del WebSocket va en la cabecera `Authorization` del saludo, **nunca** en la URL (quedaría en los logs de CloudFront y del balanceador).
  - El Excel da por hecho el WebSocket con Redis pub/sub; aquí queda como decisión del revisor (§7) por el costo en Upstash.

## 4. Modelo de datos
- **Colección `mensajes`:**
  - `grupo_id`, `autor_id` (vacío en avisos del sistema), `tipo` (`texto`, `sistema`).
  - `texto` (1 a 1000 caracteres, sin espacios a los lados).
  - `reacciones[{usuario_id, emoji}]` (una por persona).
  - `id_cliente` (UUID del teléfono), `eliminado_en`, `creado_en`.
- **Avisos del sistema:** nv-39 los publica con `chat.PublicarSistema(ctx, grupoID, texto)`.
- **Índices** (`scripts/indexes`):
  - `ix_chat {grupo_id: 1, _id: -1}`.
  - `ux_id_cliente {grupo_id: 1, id_cliente: 1}`, único.
  - `ix_chat_texto {grupo_id: 1, texto: "text"}`, en español.
  - `ix_autor {autor_id: 1}`, para el borrado ARCO.
- **`grupos.ultimo_mensaje`:** `{texto (recortado a 80), autor_id, fecha}`. Se actualiza al enviar, solo si la fecha es más nueva, y se limpia si se borra ese mensaje. `GET /grupos` de nv-37 lo devuelve.
- **Idempotencia:** el mismo `id_cliente` en el mismo grupo devuelve con 200 el mensaje ya guardado. La garantiza el índice único, sin Redis.
- **Borrar:** solo el autor; vacía `texto` y `reacciones` y pone `eliminado_en` (no se guarda lo borrado). Repetirlo = 204.

## 5. Seguridad
- **Quién ve:** solo los integrantes `confirmado` del grupo; se comprueba en cada petición y al abrir el canal.
  - Quien sale, es sacado o declina deja de ver el chat y su canal se cierra.
  - Quien entra después ve el historial completo (decisión del revisor).
  - Un grupo borrado (lógico) deja de ser visible.
- **Bloqueos (nv-36):** dos personas que se bloquearon siguen viendo sus mensajes dentro de un grupo. El bloqueo no saca a nadie del grupo, igual que en planes compartidos.
- **Logs:** solo ids (`grupo_id`, `mensaje_id`), tipo de evento, tamaño y resultado. Nunca `texto`, emojis, nombres ni el token. Las métricas del canal cuentan conexiones, no personas.
- **Push (nv-41):** solo a integrantes confirmados sin canal abierto.
  - Título y texto genéricos ("Tienes mensajes nuevos en un grupo"), sin el texto ni el nombre del autor.
  - Agrupado: máximo 1 push por grupo y persona cada 5 minutos.
  - Tipo nuevo `mensaje_grupo` en nv-40.
- **Moderación:** el texto del chat no pasa por nv-42 antes de enviarse, por el costo del modelo en cada mensaje y la latencia. Decisión del revisor.
- **Límites:**
  - 30 mensajes por minuto por usuario (`rate:chat:{usuario}`, `Limiter.WindowUser`), además de la política `writes`.
  - Canal: 3 conexiones por usuario y 500 por instancia; al pasarse, 429 en el saludo.
- **ARCO (`nv-05`):**
  - `Purger.PurgeAccount`: en los mensajes del usuario vacía `texto`, pone `eliminado_en` y quita `autor_id`. También quita sus reacciones y limpia `ultimo_mensaje` si era suyo. La conversación de los demás conserva su forma ("Mensaje eliminado").
  - `PurgeHistory` no toca el chat: no es historial de uso.
  - `Exporter` "mensajes": los mensajes propios no borrados `{grupo_id, texto, creado_en}` y sus reacciones `{mensaje_id, emoji}`. Nunca mensajes de otras personas.

## 6. Tests / Gates
- **`tests/integration/chat_test.go`** (MongoDB y Redis reales):
  - Enviar y listar; paginar con `antes_de` y `despues_de`; buscar con `q`.
  - Reenvío con el mismo `id_cliente`: 200 y un solo documento.
  - Reacción: poner, cambiar y quitar; emoji fuera de la lista = 422.
  - Borrar propio; borrar ajeno = 404.
  - Integrante invitado o que salió = 404.
  - `ultimo_mensaje` en `GET /grupos`; aviso `PublicarSistema`.
  - 31.º mensaje en un minuto = 429.
  - Borrar la cuenta y exportar.
- **`TestChat_Concurrencia_MismoIdCliente`:** 10 envíos simultáneos con el mismo `id_cliente` dejan un documento.
- **`TestChat_TiempoReal_DosInstancias`** (si se aprueba el canal), con dos apps en proceso:
  - Un mensaje enviado a la instancia A llega al cliente conectado a B.
  - Sin token o con token vencido = 401.
  - Quien sale pierde el canal; el canal se cierra al vencer el token.
- **`tests/security/chat_security_test.go`:** las 5 rutas con 401 sin token, 403 en eliminación pendiente, 404 a quien no es integrante y token en la URL rechazado.
- **`TestPresupuestoRedis_Chat`:** fija los comandos por mensaje enviado y por evento repartido.
- **Otras suites:**
  - `tests/database/indexes_test.go`: índices de `mensajes`.
  - `tests/contract`: forma del mensaje y `mensajes` en la exportación.
  - Unit: DTOs, lista de emojis, recorte de `ultimo_mensaje` y eventos JSON.

## 7. Plan de entrega
### Tiempo real: costo y decisión
**Supuesto del piloto** (a medir con `scripts/medir-redis.sh`):
- 100 grupos activos al día, con 20 mensajes y 10 reacciones por grupo.
- 300 personas con el chat abierto 10 minutos al día.

Cada petición ya cuesta unos 3 comandos por el limitador global (nv-11), igual en las tres opciones. Presupuesto: 300 000 comandos al mes (el 60 % de 500 000).

| Opción | Comandos extra de Redis al mes | Dos instancias | Notas |
|---|---|---|---|
| A. Sin canal: la app consulta `despues_de` cada 5 s con el chat abierto, más push | ~3 200 000 (cada consulta pasa por el limitador) | sin cambios | Excede la cuota 10 veces. Descartada, salvo con consultas cada 60 s o más. |
| B. WebSocket + Redis pub/sub (lo que dice el Excel) | 90 000 a 270 000 (18 % a 54 %): 90 000 `PUBLISH` más hasta 180 000 entregas si Upstash las cobra | cada instancia se suscribe a `chat:{grupo}` solo mientras tiene clientes de ese grupo | Falta medir si Upstash cobra cada entrega al suscriptor; deja poco margen. |
| C. WebSocket + change streams de MongoDB Atlas | 0 | cada instancia abre un change stream sobre `mensajes` y reparte a sus clientes | Hay que comprobar que el plan de Atlas permite change streams y correr MongoDB como réplica de un nodo en Testcontainers. |

Comunes a B y C:
- **Biblioteca:** `github.com/gofiber/contrib/websocket` (dependencia nueva, pasa govulncheck y licencias).
- **Ruta:** fuera del `REQUEST_TIMEOUT` (55 s) y de la compresión. El limitador cuenta solo el saludo.
- **Conexión:** ping cada 25 s, por debajo de los 60 s de inactividad del balanceador y de CloudFront.
- **Cierre:** al vencer el token (15 min) o al revocarse (`jwt_revocado`); la app reconecta con `despues_de`.
- **Despliegue:** la instalación escalonada (nv-12) corta las conexiones de una instancia; la app reconecta a la otra.
- **Eventos JSON:** `mensaje_nuevo`, `mensaje_eliminado`, `reaccion`.

**Recomendación:** C si Atlas lo permite; si no, B con la medición. El canal es una mejora: la app siempre puede ponerse al día con `GET … ?despues_de=`. Sin decisión del revisor se entrega solo REST + push, sin la fila WS.

### Entrega
- `go build`, `go vet`, Unit y lint en local; las cinco suites en el CI del PR.
- `scripts/indexes` y las filas en `docs/endpoints.md`.
- `curl` con 401 sin token y 404 a un ajeno; tag tras aprobar.

## Aprobación
- **Estado:** 🚧 Borrador (2026-11-12), ola por decidir
- **Revisores:** Osmar Reséndiz (pendiente de aprobar)
- **Fecha:** —
- **Decisiones para el revisor:**
  1. Tiempo real: opción A, B o C de §7 (recomendada C).
  2. Quien entra después ve el historial completo.
  3. Sin moderación previa del texto.
  4. Al borrar la cuenta se vacían los mensajes en vez de borrar los documentos.
  5. 30 mensajes por minuto y 6 emojis fijos.
- **Detalle:** la evidencia irá en `specs/nv-57/checklists/gates.md` + `Memoria.md`.
