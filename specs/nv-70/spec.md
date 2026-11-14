# NV-70 — Perfil de persona, silenciar amigos y amigos en contactos

## Contexto
Tres rutas de la hoja «J2 Amigos» que ninguna spec cubre: ver el perfil de otra persona, silenciar la actividad de un amigo sin dejar de serlo y «Encuentra amigos» en la agenda del teléfono. **Origen**: filas `GET /personas/{id}`, `PUT /amistades/{id}/silencio` y `POST /personas/contactos` de la hoja «J2 Amigos», el campo `silenciado_por` de `amistades` y la regla «Amigos silenciados» de J8. **Depende de**: NV-04 (`RequireAuth`), NV-05 (`telefono_hash` con `PHONE_HMAC_KEY`, `ux_telefono_hash`, `privacidad.buscable_por_telefono`, `RequireActive`, `Purger`/`Exporter`), NV-35 (`internal/personas`, `Person`), NV-36 (`internal/amistades`, `personas.Blocks`), NV-53 (amigos en común en Neo4j con degradado) y NV-64 (`relacion` y 404 uniforme).

## Requisitos
1. `GET /personas/{id}` devuelve el perfil de otra persona con lo que su relación conmigo permite ver; un bloqueo en cualquier sentido responde igual que una cuenta inexistente.
2. `PUT /amistades/{id}/silencio` con `{silenciado}` silencia o deja de silenciar a un amigo; solo afecta a quien lo pide y el otro no se entera.
3. `POST /personas/contactos` recibe hasta 500 teléfonos de la agenda y responde quiénes tienen cuenta y aceptan ser encontrados por teléfono; nunca se guardan ni se registran los números ni la agenda.

## Diseño
### Perfil de persona (`internal/personas`)
- **Ruta** `GET /personas/:id` junto a `/personas/buscar` y `/personas/codigo/:codigo` (las rutas fijas se registran antes, para que `buscar` y `codigo` no caigan en `:id`). `id` no ObjectId = 422.
- **404 igual** (como nv-64) para: id inexistente, cuenta no activa (`estado` distinto de `activo`) y cualquier bloqueo en los dos sentidos (`personas.Blocks`). Mi propio id también da 404: el perfil propio es `GET /usuarios/yo`.
- **Lo que ve cualquiera con sesión** (lo mismo que ya muestran la búsqueda y el QR): `id`, `nombre`, `nombre_usuario`, `foto_url` y `relacion` (`amigo`, `solicitud_enviada`, `solicitud_recibida` o `ninguna`, con `personas.Blocks.Relation` de nv-64).
- **Solo si `relacion = amigo`:** `nivel` (de `usuarios.nivel`), `amigos_en_comun` (número, de Neo4j como nv-53; Neo4j caído o lento 3 s = `null` y `degradado: true`, el resto responde 200), `grupos_en_comun` (número de grupos de nv-37 donde los dos son integrantes, activos) y `silenciado` (si yo lo silencié).
- **Privacidad:** nunca correo, teléfono, `codigo_amigo`, ciudad, fecha de nacimiento, gustos ni ubicación. `buscable_por_usuario=false` no oculta el perfil a quien ya tiene el id (lo obtuvo por amistad, grupo o QR), pero un no amigo solo ve la parte básica.
- **Drift con el Excel:** «insignias» y «momentos» no existen todavía (J8 sin spec ejecutada): quedan fuera y entran con su módulo; «planes compartidos» se reduce a `grupos_en_comun` (número, sin nombres ni ids). El Excel pide 403 si hay bloqueo; se responde 404 para no revelar el bloqueo (decisión de nv-36 y nv-64).
- **Límite:** política nueva `personas_perfil`, 120 por hora por usuario (`rate:personas_perfil:{usuario}`), contra el recorrido de ids. Respuesta con `Cache-Control: no-store`.

### Silenciar a un amigo (`internal/amistades`)
- **Campo nuevo** `silenciado_por` (arreglo de ObjectId, de 0 a 2) en el documento de `amistades`, como dice la hoja J2. Sin índice nuevo: se lee junto con el par.
- **`PUT /amistades/:id/silencio`** con `{silenciado: bool}` (requerido; campo de más = 400, tipo malo = 422). Solo participante y solo en `aceptada`: no participante, inexistente o en otro estado = 404 igual (una solicitud o un bloqueo no se pueden silenciar). `true` = `$addToSet` de mi id, `false` = `$pull`; repetirlo deja lo mismo (idempotente). Responde 200 `{id, silenciado}`.
- **Sin `If-Match`:** el cambio solo toca mi propia marca y es idempotente, así que no hay carrera que perder (excepción a `AGENTS.md` §5, para el revisor).
- **Efectos:** `GET /amistades` agrega `silenciado` (mío) a cada amigo. El feed de momentos (J8, futuro) excluye a quien silencié; mientras no exista, el campo solo se guarda y se lee. No cambia avisos de amistad, grupos ni chat. Eliminar la amistad o bloquear borra la marca con el documento.
- **ARCO:** la exportación de `amistades` agrega `silenciado` por relación (solo mi marca, nunca la del otro); el `Purger` ya borra los documentos del usuario. La marca del otro sobre mí nunca se expone.

### Amigos en contactos (`internal/personas`)
- **Drift con el Excel (decisión para el revisor):** el Excel dice que la app manda los teléfonos «ya convertidos en hash». El hash guardado es HMAC-SHA-256 con `PHONE_HMAC_KEY` (secreto del servidor, nv-05) y la app no puede tener esa llave; un SHA-256 simple calculado en el teléfono se rompe probando todos los números. Por eso la app manda los teléfonos en el cuerpo (TLS) y el servidor calcula la HMAC en memoria, igual que la búsqueda por teléfono de nv-35.
- **Entrada** `{telefonos: [string]}`, de 1 a 500. Cada uno pasa por `account.NormalizePhone` y debe quedar E.164 (10 dígitos sin lada = +52, como nv-35); los inválidos se ignoran (la agenda trae basura) y si ninguno es válido = 422. Repetidos se cuentan una vez. Campo de más = 400.
- **Consulta:** una sola por `ux_telefono_hash` con `telefono_hash: {$in: [hmac…]}`, `estado: activo` y `privacidad.buscable_por_telefono: true`, sin mi cuenta; luego se quitan los bloqueos en los dos sentidos con una consulta a `amistades` por `par` (`personas.Blocks`). Cada resultado lleva su `relacion`.
- **Respuesta** 200 `{resultados[{indice, persona{id, nombre, nombre_usuario, foto_url}, relacion}]}`: `indice` es la posición en el arreglo enviado (el primero si se repitió), para que la app ponga el nombre del contacto sin que el servidor devuelva el número. Nunca teléfono ni `telefono_hash`. `Cache-Control: no-store`.
- **Nunca se guarda ni se registra:** ni números, ni hashes, ni el tamaño de la agenda en logs; la petición no crea documentos. El cuerpo no pasa por el log de acceso (`slog` no registra cuerpos) y el texto de error nunca incluye el número. `POST` porque la lista no cabe en la URL y no debe quedar en el log.
- **Límite:** política nueva `personas_contactos`, 10 por día por usuario (`rate:personas_contactos:{usuario}`, ventana de 24 h): hasta 5 000 números al día, contra la enumeración de teléfonos. Cuesta lo mismo que las demás ventanas por usuario; si `TestPresupuestoRedis_*` sube, se rehace la estimación de nv-11.

- **Variables:** ninguna nueva (`PHONE_HMAC_KEY` ya existe); los límites son constantes como en nv-35.

## Contratos
Filas del Excel (hoja «J2 Amigos») copiadas y corregidas:

| Método | Ruta | Auth | Respuestas |
|---|---|---|---|
| GET | `/api/v1/personas/{id}` | JWT | 200, 401, 403, 404, 422, 429 |
| PUT | `/api/v1/amistades/{id}/silencio` | JWT, participante | 200, 400, 401, 403, 404, 422 |
| POST | `/api/v1/personas/contactos` | JWT | 200, 400, 401, 403, 413, 422, 429 |

- `GET /personas/{id}` → `{id, nombre, nombre_usuario, foto_url, relacion}`; si `relacion = amigo`, además `{nivel, amigos_en_comun, grupos_en_comun, silenciado, degradado}`. Excel: «200 o 403 si está bloqueado» → 404 igual.
- `PUT /amistades/{id}/silencio` con `{silenciado}` → `{id, silenciado}`.
- `POST /personas/contactos` con `{telefonos[]}` (Excel: `hashes[]`, máx. 500) → `{resultados[{indice, persona, relacion}]}`.

403 = cuenta en eliminación pendiente (`RequireActive`). Las tres filas pasan a `docs/endpoints.md` y las dos políticas a `docs/rate-limit-matrix.md` al ejecutar la spec (no en este PR).

## Tests
- `tests/integration/personas_perfil_test.go`: perfil básico de un no amigo, perfil completo de un amigo con `amigos_en_comun` y `grupos_en_comun`, Neo4j caído = `degradado` con 200, propio id, inexistente, inactivo y bloqueado en ambos sentidos = 404 igual, id malo = 422, 429 en la 121.ª.
- `tests/integration/amistades_silencio_test.go`: silenciar y quitar (idempotente), solo cambia mi marca, solicitud pendiente y no participante = 404, `GET /amistades` trae `silenciado`, borrar la amistad borra la marca.
- `tests/integration/personas_contactos_test.go`: encuentra solo cuentas activas con `buscable_por_telefono`, sin la propia ni bloqueadas, `indice` correcto con repetidos, inválidos ignorados, todos inválidos = 422, 501 teléfonos = 422, 429 en la 11.ª del día, y ningún documento nuevo en ninguna colección tras la petición.
- `tests/security/personas_privacidad_test.go`: 401 en las tres rutas, 403 con cuenta en eliminación, el perfil de un no amigo sin `nivel` ni conteos, ningún correo, teléfono, `telefono_hash` ni `codigo_amigo` en las respuestas, ningún número ni hash en los logs capturados, y la marca de silencio del otro nunca expuesta.
- `tests/integration/presupuesto_redis_test.go` (`TestPresupuestoRedis_VentanasPorUsuarioNV70`): las dos políticas nuevas cuestan lo mismo que `personas_buscar`.
- `tests/contract/personas_perfil_contract_test.go`: 200 y 404 del perfil, 200 del silencio, 200 de contactos.
- Unit: forma del perfil según `relacion`, normalización y dedupe con `indice`, validator de `silenciado` (`internal/personas`, `internal/amistades`).

## Criterios
- Un no amigo nunca ve más de lo que ya muestra la búsqueda, un bloqueo es indistinguible de una cuenta inexistente y ningún número de la agenda queda guardado ni en logs.

## Límites
- Sin insignias ni momentos en el perfil (llegan con J8 y gamificación).
- Sin guardar la agenda ni avisar a nadie de que «un contacto se unió».
- Sin `GET /personas/yo/qr` (fuera desde nv-64).

## Aprobación y Control de Cambios
- **Estado:** 🚧 Borrador (2026-11-13), ola por decidir
- **Revisores:** Osmar Reséndiz (pendiente de aprobar)
- **Fecha:** —
- **Decisiones para el revisor:** teléfonos en claro por TLS y HMAC en el servidor (no `hashes[]` del Excel); `indice` en vez de devolver el número; contactos 10 por día y perfil 120 por hora; bloqueo = 404 (no 403); perfil completo solo para amigos y sin insignias ni momentos; silencio sin `If-Match`.
- **Detalle:** evidencia en `specs/nv-70/checklists/gates.md` + `Memoria.md` al ejecutar.
