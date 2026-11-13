# NV-64 — Reporte de errores de la app y búsqueda por código de amigo

## Contexto
Dos rutas pequeñas que faltan del Excel: (a) la app reporta errores agrupados que Crashlytics no ve (pagos, sincronización) y el administrador los consulta; (b) al escanear el QR de amistad, la app busca a la persona por su `codigo_amigo`. **Origen**: hoja «J9 Salud y monitoreo» (Roles D10 y D6, US-169) y fila `GET /personas/codigo/{codigo}` de la hoja «J2 Amigos» (Roles C4). **Depende de**: NV-04 (`RequireRoles`, rol `administrador`), NV-05 (`codigo_amigo`, `ux_codigo_amigo`, `RequireActive`), NV-13 (logs en CloudWatch), NV-17 (cursor firmado), NV-35 (`internal/personas`, `Person`) y NV-36 (bloqueos, `personas.Blocks`).

## Requisitos
1. `POST /errores` recibe un error de la app sin datos personales y lo agrupa por huella; responde 202.
2. `GET /admin/errores` lista los errores agrupados del más reciente al más viejo, con filtro por módulo y fecha; solo administrador.
3. Los errores se borran solos a los 30 días de su última aparición.
4. `GET /personas/codigo/{codigo}` devuelve el perfil público de quien tiene ese código, o 404.

## Diseño
### Errores de la app
- **Paquete `internal/errores`:** repositorio, service, handler y DTO.
- **Colección `errores_app`** (Excel J9 §1): `origen`, `version`, `modulo`, `mensaje`, `traza_id`, `conteo`, `huella`, `creado_en` (primera vez) y `ultimo_en`. Índices `ux_huella` (único), `ix_errores_modulo` (`modulo`, `ultimo_en` descendente) y `ttl_errores` (`ultimo_en`, `expireAfterSeconds: 2592000`), en `scripts/indexes` y `/ready`. Sin `usuario_id`: no hay nada que purgar ni exportar en ARCO.
- **Entrada** (validators de nv-17; las reglas nuevas `modulo`, `semver` y `traceid` se registran en `internal/shared/validation`): `modulo` `^[a-z_]{2,40}$`, `mensaje` de 1 a 500 caracteres, `version` SemVer de hasta 20, `traza_id` opcional de hasta 64 `[A-Za-z0-9-]`. Campo de más = 400; formato malo = 422.
- **Sin datos personales:** antes de guardar, `mensaje` pasa por el filtro del logger (correos, llaves, credenciales en URI), que se exporta como `logger.RedactText`, y además se enmascaran secuencias de 7 o más dígitos (teléfonos, tarjetas) y tokens con forma de JWT. La huella se calcula sobre el texto ya filtrado.
- **Agrupar:** `huella` = SHA-256 hex de `modulo` + `mensaje` filtrado (no es contraseña, §3 lo permite). `upsert` con `$inc conteo`, `$set ultimo_en`, `version` y `traza_id`, y `$setOnInsert creado_en`. 202 tras escribir; MongoDB caído = 503 uniforme.
- **Drift con el Excel:** `origen` queda fijo en `app`; los errores del servidor siguen en CloudWatch (nv-13) y no se copian aquí. `traza_id` es el `request_id` de la API, no un id de Loki. «Más frecuentes» y «del más reciente al más viejo» chocan en el Excel: se ordena por `ultimo_en` (el índice existe) y cada fila trae `conteo`.
- **Límite:** política nueva `errores_reporte`, 30 por hora por usuario (`Limiter.WindowUser`, llave `rate:errores_reporte:{usuario}`), en `docs/rate-limit-matrix.md`. Cuesta lo mismo que las demás ventanas por usuario; si `TestPresupuestoRedis_*` sube, se rehace la estimación de nv-11.
- **Administrador:** `modulo` opcional, `desde` (RFC 3339, filtra `ultimo_en ≥ desde`), cursor firmado de nv-17 (`Signer.Scope("admin_errores", modulo, desde)`), `limite` 20 por omisión y 100 como máximo. Comparte la política `admin` (200 por minuto por IP, `rate:admin:{ip}`) con las demás rutas admin (nv-20, nv-43), antes de `RequireRoles`.
- **Auth del reporte:** JWT de cualquier rol con `RequireAuth`, sin `RequireActive` (una cuenta en eliminación también puede fallar).

### Búsqueda por código de amigo
- **En `internal/personas`:** `GET /personas/codigo/{codigo}` junto a `/personas/buscar`; mismo `Person` (`id`, `nombre`, `nombre_usuario`, `foto_url`) más `relacion`: `amigo`, `solicitud_enviada`, `solicitud_recibida` o `ninguna` (desde `amistades`). `personas.Blocks` gana `Relation(me, other)` (estado y si la creó quien pregunta), que cumple `amistades.Repository` con una sola consulta por `par`.
- **Código:** se pasa a mayúsculas y se valida contra el alfabeto y largo de `usuarios` (8 de `23456789ABCDEFGHJKMNPQRSTUVWXYZ`); otro formato = 422. Busca con `ux_codigo_amigo` (`usuarios.Repository.FindByFriendCode`; el formato lo valida `usuarios.ValidFriendCode`).
- **404 igual** para: código inexistente, cuenta no activa, mi propio código y cualquier bloqueo en los dos sentidos (`personas.Blocks`). `buscable_por_usuario=false` no lo oculta: compartir el QR es explícito.
- **Límite:** política nueva `personas_codigo`, 30 por hora por usuario (`rate:personas_codigo:{usuario}`), contra la enumeración de códigos.
- **Fuera:** `GET /personas/yo/qr` (la app dibuja el QR con el `codigo_amigo` de `GET /usuarios/yo`).

- **Variables:** ninguna nueva; los límites son constantes como en nv-35 (nada que agregar a Parameter Store).

## Contratos
| Método | Ruta | Auth | Respuestas |
|---|---|---|---|
| POST | `/api/v1/errores` | JWT | 202, 400, 401, 422, 429, 503 |
| GET | `/api/v1/admin/errores?modulo=&desde=&cursor=&limite=` | JWT administrador | 200, 401, 403, 422, 429, 503 |
| GET | `/api/v1/personas/codigo/{codigo}` | JWT | 200, 401, 403, 404, 422, 429 |

403 en `/personas/codigo` = cuenta en eliminación pendiente. Las dos rutas de errores salieron de «Diferidos» y las tres están en `docs/endpoints.md`; las dos políticas nuevas, en `docs/rate-limit-matrix.md`.

## Tests
- `tests/integration/errores_test.go`: el mismo error dos veces suma `conteo` en un documento sin `usuario_id`, el filtro quita correo, teléfono y JWT, 400 y 422, lista del administrador por módulo, `desde` y cursor (otro filtro = 422; consumidor = 403), 429 en el 31.º reporte y 401 sin token.
- `tests/integration/personas_codigo_test.go`: encontrado con `relacion`, minúsculas aceptadas, inexistente, propio, bloqueado en ambos sentidos y cuenta no activa dan 404; 429 en la 31.ª.
- `tests/security/errores_security_test.go`: 401 en las tres rutas, 403 de consumidor y comerciante en `/admin/errores`, 202 al reportar y 403 en `/personas/codigo` con cuenta en eliminación, ningún correo, tarjeta, teléfono, credencial de URI, llave de Google, JWT ni id de quien reporta en `errores_app`, y el perfil por código sin correo, teléfono ni código, con cuenta bloqueada o en eliminación igual a inexistente.
- `tests/integration/presupuesto_redis_test.go` (`TestPresupuestoRedis_VentanasPorUsuarioNV64`): las dos políticas nuevas cuestan lo mismo que la ventana de `personas_buscar`.
- `tests/database/indexes_test.go`: índices, `ux_huella` único y TTL de 30 días de `errores_app`. `tests/contract/errores_contract_test.go`: 202 del reporte, 200 de la lista, 200 y 404 por código.
- Unit: validators, huella estable, filtro de datos personales y service con repositorio que registra (`internal/errores/service_test.go`); formato del código, `relacion` y 404 uniforme (`internal/personas/codigo_test.go`).

## Criterios
- Ningún correo, teléfono o token llega a `errores_app`, y un código ajeno a mi red solo revela lo que ya muestra la búsqueda de personas.

## Límites
- Sin `GET /metrics` (sigue diferido) ni errores del servidor en `errores_app`.
- Sin alarma en CloudWatch por errores de la app.
- Sin `GET /personas/yo/qr`.

## Aprobación y Control de Cambios
- **Estado:** 🚧 Borrador con evidencia (2026-11-13), implementada en la rama `nv-64`; ola 8
- **Revisores:** Osmar Reséndiz (pendiente de aprobar)
- **Fecha:** —
- **Decisiones para el revisor:** `origen` solo `app`; orden por `ultimo_en`; dos políticas nuevas de 30/h (cuota de Upstash); `/admin/errores` en la política `admin`; reporte sin `RequireActive`; propio código = 404; el QR ignora `buscable_por_usuario`.
- **Detalle:** evidencia en `specs/nv-64/checklists/gates.md` + `Memoria.md` al ejecutar.
