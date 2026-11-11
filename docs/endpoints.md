# Endpoints — vivo (nv-01, nv-04, nv-05, nv-20, nv-30)

> Fuente de contratos. Las specs solo enlazan, no duplican. Constitución §6.
> Las filas de módulos de negocio se agregan con su spec (`nv-04+`), copiadas de `Wherever_Endpoints_CRUD.xlsx` (hojas A1–A7, J1–J9).

| Método | Ruta | Auth | Rate | Códigos | Spec |
|---|---|---|---|---|---|
| GET | `/health` → `{estado:"ok", version}` | no | — | 200 | nv-01, nv-11 |
| GET | `/ready` → `{estado, dependencias}` (MongoDB, Redis, Neo4j e `indices_mongodb`; 1 s c/u; Redis caído = 200 `degraded`; falta un índice declarado = 503 hasta correr `scripts/indexes`) | no; también público en staging, sin direcciones | global | 200/503 | nv-01, nv-04 |
| GET | `/api/v1/ping` | no | global | 200 | nv-01 |
| GET | `/api/v1/probe/timeout` (no-prod) | no | global | 408/404 | nv-01 |
| GET | `/api/v1/probe/error` (no-prod) | no | global | 500/404 | nv-01 |
| GET | `/api/v1/probe/forbidden` (no-prod) | no | global | 403/404 | nv-01 |
| GET | `/api/v1/probe/conflict` (no-prod) | no | global | 409/404 | nv-01 |
| GET | `/api/v1/probe/invalid` (no-prod) | no | global | 422/404 | nv-01 |
| GET | `/api/v1/probe/lenta` (no-prod): tarda 45 s | no | global | 200/408/404 | nv-12 |
| GET | `/docs` y `/openapi.json` (solo dev) | no | — | 200/404 | nv-01 |
| POST | `/api/v1/auth/login` → `{correo, password, dispositivo{modelo, sistema, app_version}}` → tokens. Argon2id; 5 fallos del mismo correo (exista o no) → 423 con minutos restantes | no | login 5/5 min por IP | 200/400/401/422/423/429/503 | nv-04 (J1) |
| POST | `/api/v1/auth/refresh` → `{refresh_token}` → tokens nuevos; rota el refresh y uno ya rotado revoca la familia | refresh_token | 10/min por IP | 200/400/401/422/429/503 | nv-04 (J1) |
| POST | `/api/v1/auth/logout` → cierra la sesión actual (`revocada_en` + `jwt_revocado:{jti}`) | JWT | global | 204/401/503 | nv-04 (J1) |
| POST | `/api/v1/auth/logout-todos` → cierra todas las sesiones del usuario | JWT | global | 204/401/503 | nv-04 (J1) |
| GET | `/api/v1/auth/sesiones` → `[{id, dispositivo, ultimo_uso, actual}]` (solo abiertas y no vencidas) | JWT | global | 200/401 | nv-04 (J1) |
| DELETE | `/api/v1/auth/sesiones/{id}` → cierra una sesión propia; ajena o inexistente = 404 | JWT, dueño | global | 204/401/404 | nv-04 (J1) |
| POST | `/api/v1/auth/recuperar` → `{correo}`; enlace de 1 h; siempre 202 exista o no el correo; no reenvía antes de 60 s | no | 10/min por IP | 202/400/422/429 | nv-04 (J1) |
| POST | `/api/v1/auth/restablecer` → `{token, password_nuevo, dispositivo?}` → tokens; enlace de un solo uso; cierra las otras sesiones | token del enlace | 10/min por IP | 200/400/422/429 | nv-04 (J1) |
| POST | `/api/v1/auth/verificar-correo` → `{token}` → `{correo_verificado: true}`; enlace de 24 h, un solo uso | token del enlace | 10/min por IP | 200/400/422/429 | nv-04 (J1) |
| POST | `/api/v1/auth/reenviar-verificacion` → reenvía el enlace (máx. 1 cada 60 s, `Retry-After`) | JWT | global | 202/401/429 | nv-04 (J1) |
| PUT | `/api/v1/auth/password` → `{password_actual, password_nuevo}`; cierra las otras sesiones | JWT | global | 204/400/401/422 | nv-04 (J1) |
| POST | `/api/v1/usuarios` → `{nombre, fecha_nacimiento (AAAA-MM-DD), correo, password, acepta_terminos, version_terminos, promociones_correo, dispositivo?}` → 201 con tokens; 18+ por fecha UTC, rol `consumidor`, 50 puntos, manda verificación | no | registro 5/h por IP | 201/400/409/422/429 | nv-05 (A1) |
| GET | `/api/v1/usuarios/correo-disponible` con `?correo=` → `{disponible}` (usa `ux_correo`) | no | 20/min por IP | 200/400/422/429 | nv-05 (A1) |
| GET | `/api/v1/usuarios/yo` → perfil completo (sin `password_hash`, `telefono_hash` ni `google_id`) con `version` y `ETag` | JWT | global | 200/401 | nv-05 (A1) |
| PATCH | `/api/v1/usuarios/yo` → `{nombre?, foto_url? (https), ciudad_id?}` con `If-Match: "<version>"`; sin `If-Match` 400, versión vieja 409 | JWT, dueño | global | 200/400/401/409/422 | nv-05 (A1) |
| PUT | `/api/v1/usuarios/yo/correo` → `{correo_nuevo, password}`; queda sin verificar y manda verificación | JWT, dueño | global | 200/400/401/409/422 | nv-05 (A1) |
| PUT | `/api/v1/usuarios/yo/gustos` → `{categorias[3..10], presupuesto_min, presupuesto_max, restricciones[], alergias}`; `alergias` no vacío exige `privacidad.datos_sensibles` (422 si no, US-010); no suma puntos (se dan al registrarse) | JWT consumidor | global | 200/400/401/403/422 | nv-05 (A1) |
| PATCH | `/api/v1/usuarios/yo/preferencias` → parcial `{movilidad_default, tema, unidades, horario_silencioso{desde, hasta}, notificaciones{}}`; `reservas: false` = 422 | JWT, dueño | global | 200/400/401/422 | nv-05 (A1) |
| PATCH | `/api/v1/usuarios/yo/privacidad` → parcial `{ubicacion, aprendizaje_personalizado, estado_animo, datos_sensibles, datos_uso, buscable_por_telefono, promociones_correo, version_terminos}` | JWT, dueño | global | 200/400/401/422 | nv-05 (A1) |
| PUT | `/api/v1/usuarios/yo/telefono` → `{telefono}`; se quitan espacios, guiones, puntos y paréntesis y debe quedar E.164 (`+` y lada del país); se guarda solo su HMAC-SHA-256; repetido = 409 | JWT, dueño | global | 200/400/401/409/422 | nv-05 (A1) |
| POST | `/api/v1/usuarios/yo/modo` → `{modo: explorar|negocio}`; negocio sin rol `comerciante` = 403 | JWT | global | 200/400/401/403/422 | nv-05 (A1) |
| POST | `/api/v1/usuarios/yo/exportar` → 202; una por día (429 con `Retry-After`); la arma el trabajo nocturno y manda un enlace de 24 h (D-2) | JWT, dueño | global | 202/401/429 | nv-05 (A1) |
| POST | `/api/v1/usuarios/exportaciones/descargar` → `{token}` → el JSON de "mis datos"; enlace de un solo uso (D-2); al descargarlo el archivo se borra de la base | token del enlace | 10/min por IP | 200/400/410/422/429 | nv-05 (A1) |
| DELETE | `/api/v1/usuarios/yo/historial` → borra el historial de todos los módulos registrados; conserva la cuenta | JWT, dueño | global | 204/401/503 | nv-05 (A1) |
| DELETE | `/api/v1/usuarios/yo` → `{confirmacion: "ELIMINAR"}` → 202 `{eliminar_en}` (hoy + 30 días); cierra todas las sesiones | JWT, dueño | global | 202/400/401/422 | nv-05 (A1) |
| POST | `/api/v1/usuarios/yo/cancelar-eliminacion` → perfil; fuera de plazo o sin eliminación pendiente = 409 | JWT, dueño | global | 200/401/409 | nv-05 (A1) |
| GET | `/api/v1/ciudades/autocompletar` con `?q=&limite=` → `[{id, nombre, estado}]`; `q` de 2 letras o más, sin acentos; `limite` 8 por defecto, máx. 50; primero las de cobertura | no | global | 200/400/422 | nv-05 (A1) |
| GET | `/api/v1/categorias` → `{categorias[{id,nombre,conteo}], etiquetas[{id,nombre,conteo}], servicios[{id,nombre,conteo}], filtros{nivel_precio[], opciones_dieta[], accesibilidad[]}}`; 6 giros aunque tengan 0; sin token, sin `ETag` y `Cache-Control: no-store` hasta NV-18 | no | global | 200/503 | nv-20 (A2) |
| GET | `/api/v1/lugares` con `?categoria=&precio_max=&abierto_ahora=&lat=&lng=&radio_m=&orden=&cursor=&limite=` → `{items[{id,slug,nombre,categoria,nivel_precio,precio_promedio,calificacion{promedio,total_resenas},foto_portada,ubicacion{lat,lng},distancia_m?,abierto_ahora}], siguiente_cursor, limite}`; `lat` y `lng` juntos, `radio_m` 5,000 por defecto y 20,000 máx.; `orden` `distancia` (por defecto con ubicación), `calificacion` (sin ella) o `precio` | opcional | global | 200/401/422 | nv-21 (A2) |
| GET | `/api/v1/lugares/mapa` con `?min_lat=&min_lng=&max_lat=&max_lng=&zoom=&categoria=` → `{agrupado, pines[{id,nombre,categoria,calificacion,ubicacion}], celdas[{centro{lat,lng},conteo}]}`; hasta 100 pines; si hay más, celdas de `360/2^zoom/4` grados (máx. 400); `zoom` 3–20, ancho < 180° | no | global | 200/422 | nv-21 (A2) |
| GET | `/api/v1/lugares/{id}` → ficha pública con `abierto_ahora` (hora de `America/Mexico_City`); sin dueño, QR, plan, revisión ni versión; no aprobado, eliminado o id mal formado = 404 | no | global | 200/404 | nv-21 (A2) |
| POST | `/api/v1/planes` → `{nombre, fecha (día), personas, presupuesto, movilidad, origen?, paradas[{negocio_id, llega, minutos, precio, nota?}]}` (1 a 6 paradas); responde el plan calculado con `ETag` | JWT | global | 201/400/401/403/422 | nv-30 (A4) |
| GET | `/api/v1/planes` con `?pestana=&cursor=&limite=`; `pestana` es `proximos` (por defecto), `guardados` o `pasados`; solo los planes activos de quien llama | JWT | global | 200/401/422 | nv-30 (A4) |
| GET | `/api/v1/planes/{id}` → plan con `traslados`, `costo_total`, `costo_por_persona` y `version`; ajeno o borrado = 404 | JWT, dueño | global | 200/401/404 | nv-30 (A4) |
| PATCH | `/api/v1/planes/{id}` con `If-Match` → cambia nombre, fecha (mueve las paradas), personas, presupuesto o movilidad | JWT, dueño | global | 200/400/401/403/404/409/422 | nv-30 (A4) |
| DELETE | `/api/v1/planes/{id}` con `If-Match` → borrado lógico | JWT, dueño | global | 204/400/401/403/404/409 | nv-30 (A4) |
| POST | `/api/v1/planes/{id}/clonar` → `{fecha, nombre?}`; plan nuevo con las mismas horas en el día nuevo, sin reservas ni integrantes | JWT, dueño | global | 201/401/403/404/422 | nv-30 (A4) |
| POST | `/api/v1/planes/{id}/paradas` con `If-Match` → agrega una parada | JWT, dueño | global | 201/400/401/403/404/409/422 | nv-30 (A4) |
| PUT | `/api/v1/planes/{id}/paradas/orden` con `If-Match` → `{orden[ids]}`; reparte las mismas horas de llegada en el orden pedido | JWT, dueño | global | 200/400/401/403/404/409/422 | nv-30 (A4) |
| PATCH | `/api/v1/planes/{id}/paradas/{parada_id}` con `If-Match` → mueve la parada o cambia minutos, precio o nota | JWT, dueño | global | 200/400/401/403/404/409/422 | nv-30 (A4) |
| DELETE | `/api/v1/planes/{id}/paradas/{parada_id}` con `If-Match` → quita la parada (la última no, 422) | JWT, dueño | global | 200/400/401/403/404/409/422 | nv-30 (A4) |
| POST | `/api/v1/admin/negocios/importar` → multipart `archivo` `.csv` o `.json` o arreglo JSON de 1–100; idempotente por `slug`; responde `{creados, actualizados, errores[{fila,campo,mensaje}]}` y los válidos quedan `aprobado` | JWT administrador | admin 200/min | 200/400/401/403/413/422/503 | nv-20 (A2/A5) |

Convención JSON: `snake_case`. `PageResult{items, siguiente_cursor, limite}` (desde nv-02). Errores `{codigo, mensaje, campos[], detalle?, traza_id}`.

Tokens (nv-04): `{access_token, refresh_token, expira_en, usuario{id, nombre, correo, correo_verificado, roles}}`; `expira_en` = vencimiento del access token (ISO 8601 UTC, 15 min); refresh opaco de 30 días. Contraseña nueva: 8 a 128 caracteres con letra y número (422 si no). Cuerpo ilegible = 400.

Sondas y `detalle` (nv-15): solo fuera de prod y sin `HIDE_INTERNALS=true`. El servidor público de AWS los oculta: `/probe/*` da 404 y los errores no traen `detalle`.

Consentimiento (nv-15): al retirar `datos_sensibles` se borran las alergias guardadas.

Concurrencia (nv-05): `GET /usuarios/yo` y cada cambio devuelven `version` y `ETag`. `PATCH /usuarios/yo` exige `If-Match: "<version>"`: sin la cabecera = **400**; con una versión vieja = **409**, que la app debe mostrar como "tus datos cambiaron en otro dispositivo, recarga" y volver a pedir `GET /usuarios/yo` (la constitución §5 no usa 412 ni 428).

Cuenta en eliminación pendiente (nv-05): puede iniciar sesión, pero con sesión solo responden `GET /usuarios/yo`, `POST /usuarios/yo/cancelar-eliminacion`, `POST /auth/logout` y `POST /auth/logout-todos`; las demás rutas con JWT dan **403** hasta cancelar.

Catálogo de negocios (nv-20): `GET /categorias` no pagina, no lleva token y solo cuenta `estado=aprobado` sin `eliminado_en`; devuelve `Cache-Control: no-store` y ningún `ETag` hasta NV-18. La semilla (A5) usa el encabezado de `docs/semilla-negocios.csv` o un arreglo JSON: BOM tolerado, listas con `|`, 3–10 fotos, 1–100 filas y `slug` único; las filas válidas quedan `aprobado` y los errores no detienen el archivo. `propietario_correo` es opcional: si falta, el administrador que importa queda como dueño provisional.

Lugares (nv-21): todo filtro (`estado=aprobado` sin `eliminado_en`, categoría, precio, abierto ahora y radio) va dentro de MongoDB antes del límite. «Abierto ahora» usa `horario_min[]` de nv-20: abierto si hay ventana de hoy con `desde ≤ minuto < hasta` o la de ayer con `hasta > minuto + 1440`. El cursor guarda `orden:valor` y el `_id` del último; por distancia la página siguiente usa `minDistance` y desempata por `_id`. Un cursor de otro orden es 422. La ubicación de la consulta no se guarda ni se registra.

## Diferidos (sin ruta en código todavía)
- `POST /api/v1/auth/google` (`{id_token, dispositivo{}}` → 200 con tokens, 201 si es cuenta nueva) — nv-04 T7, espera la decisión D-1 (cuenta sin `fecha_nacimiento`) y la excepción de stack `google.golang.org/api/idtoken`. Hoja J1.
- `GET /metrics` (Prometheus, red interna) — spec de observabilidad. Hoja J9.
- `POST /api/v1/errores` y `GET /api/v1/admin/errores` (colección `errores_app`) — spec de observabilidad. Hoja J9.
