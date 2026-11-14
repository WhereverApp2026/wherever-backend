# NV-68 — Extras de planes: alternativas, cambiar parada, populares y paquete sin conexión

## Contexto
Rutas de la hoja «A4 Planes» del Excel que ninguna spec cubría. **Origen**: pantallas P-16, P-17 y Inicio; US-044, US-058, US-089 y US-178. **Depende de**: NV-17 (validadores y `Signer`), NV-29 (modelo de plan y `Calculadora`), NV-30 (endpoints de planes, `If-Match`), NV-31 a NV-33 (`negocios.Candidates`, filtros duros y abiertos a la hora), NV-36 (bloqueos), NV-37 y NV-38 (grupos, integrantes y vista pública) y NV-46 (`sync_seq` de la descarga de cambios). Las reservas del paquete llegan con NV-59; el filtro del modo sorpresa, con NV-69.

## Requisitos
1. `GET /planes/{id}/paradas/{parada_id}/alternativas` devuelve hasta 3 negocios parecidos a la parada, abiertos a su hora; con `q`, busca por texto.
2. `POST /planes/{id}/paradas/{parada_id}/cambiar` reemplaza el negocio de la parada y recalcula el plan con control de versión.
3. `GET /planes/populares` muestra 3 planes públicos populares de la semana cerca de un punto, sin datos de sus dueños y sin sesión.
4. `GET /planes/{id}/paquete-offline` entrega en una respuesta todo lo que la app necesita para el plan del día sin señal.
5. La vista pública por enlace (`GET /planes/compartidos/{codigo}` del Excel) ya existe en NV-38; esta spec no agrega otra ruta.

## Diseño
### Alternativas y cambiar parada
- **Quién:** solo el dueño (`getOwned`, igual que las demás ediciones de NV-30). En NV-37 el organizador es siempre el dueño del plan, así que «dueño u organizador» del Excel es la misma persona. Integrante o ajeno = 404.
- **Alternativas sin `q`:** `negocios.Candidates` alrededor de la parada actual (radio 3 km, el de NV-31), con los filtros duros de NV-31: aprobado y visible, misma `categoria`, `nivel_precio` a ±1 y abierto en `[llega, llega+minutos]` según el horario del día. Se excluyen los negocios que ya están en el plan. Orden por distancia; 3 como máximo; puede haber menos (200 con lista corta, nunca 404).
- **Alternativas con `q`:** «Buscar otra cosa» reutiliza la búsqueda de lugares (`negocios.Service.List` con `q`), con el mismo filtro de abiertos a la hora y la misma exclusión. `q` de 2 a 60 caracteres.
- **Respuesta:** tarjetas públicas de `GET /lugares` (`FindCards`) más `distancia_m` y `minutos_extra` (diferencia estimada del traslado con la `Calculadora`). Sin IA: cero llamadas a Gemini.
- **Cambiar:** cuerpo `{negocio_id}` y versión en `If-Match` (convención de NV-30; el Excel la ponía en el cuerpo). El negocio debe estar aprobado, visible, abierto a la hora y fuera del plan (422 si no). Conserva `id`, `llega` y `minutos` de la parada, toma `precio` del negocio y pasa por `Calculadora.Aplicar`; `version` sube 1 y se marca `sync_seq` (NV-46). Versión vieja = 409; sin `If-Match` = 400.
- **Límite:** las dos rutas usan la política de escritura de planes; alternativas cuenta aparte como `planes_alternativas`, 60 por hora por usuario (`rate:planes_alternativas:{usuario}`), porque cada llamada hace un `$geoNear`.

### Planes populares
- **Drift con el código:** `planes` no tiene `visibilidad`, `veces_clonado` ni `zona`, y `POST /planes/{id}/clonar` solo copia planes propios. Esta spec agrega:
  - `visibilidad`: `privado` (por omisión, también en los planes que ya existen) o `publico`, editable con `PATCH /planes/{id}`. `publico` lo pueden ver y copiar otras personas con sesión; nunca expone al dueño. NV-69 usa este campo para «Me apunto».
  - `veces_clonado` (int, 0) y `plan_origen_id` (opcional), y `origen` gana el valor `clonado`.
  - `zona` (GeoJSON `Point`): la ubicación del negocio de la primera parada, que fija `Calculadora.Aplicar`.
- **Copiar un plan público:** `POST /planes/{id}/clonar` acepta también un plan `publico` ajeno no borrado y sin bloqueo con su dueño (NV-36; bloqueado = 404). La copia es `privado`, `origen: clonado`, con `plan_origen_id`, sin reservas ni grupo, y el original recibe `$inc veces_clonado` (una sola operación atómica, sin leer y escribir). `GET /planes/{id}` de un plan `publico` ajeno devuelve la vista sin dueño, igual que la de NV-38.
- **Consulta:** `lat` y `lng` obligatorios, redondeados a 2 decimales (≈1 km) antes de usarlos: no se guarda ni se registra la posición exacta. `$geoNear` sobre `zona` (radio 15 km) con `visibilidad: publico`, `estado: activo` y `actualizado_en` de los últimos 7 días, orden por `veces_clonado` descendente, 3 resultados.
- **Respuesta pública:** `id`, `nombre`, `personas`, `movilidad`, `costo_por_persona`, `veces_clonado` y `paradas[{lugar (tarjeta), llega, minutos}]`. Sin dueño, integrantes, notas ni grupo. Sin sesión. El `id` no es dato personal y solo sirve, con sesión, para leer o copiar el plan público.
- **Dos instancias y cuota:** sin caché en memoria ni en Redis (gastaría cuota de Upstash). `Cache-Control: public, max-age=300`: como el punto va redondeado, CloudFront y el teléfono reutilizan la respuesta entre personas cercanas. Límite por IP con una política nueva `planes_populares`, 60 por minuto (`rate:planes_populares:{ip}`), antes de la consulta.
- **Índice:** `ix_populares` = `{zona: "2dsphere", visibilidad: 1, veces_clonado: -1}` (el del Excel no tenía la parte geográfica), en `scripts/indexes` y `/ready`.

### Paquete sin conexión
- **Quién:** las mismas reglas que `GET /planes/{id}` (dueño, o integrante invitado o confirmado del grupo, NV-37). Ajeno = 404.
- **Ventana:** solo si la `fecha` del plan es hoy o mañana en la zona del usuario (la app lo pide 2 h antes; el servidor no programa nada). Fuera de la ventana = 422 `fuera_de_ventana`.
- **Contenido:** el plan completo; por cada negocio, la ficha de `GET /lugares/{id}` (nombre, dirección, ubicación, teléfono del negocio, foto principal, horario del día); el `punto_encuentro` del grupo si existe (NV-69); y, cuando exista NV-59, las reservas propias del plan con su código y el contenido del QR. Además `generado_en`, `version` y `sync_seq` para que la app sepa desde dónde seguir con `GET /sincronizacion` (NV-46).
- **Sin imágenes binarias:** solo URLs de CloudFront; la app las guarda al descargar.
- **Caché:** `ETag` = `"{version}-{sync_seq}"`; con `If-None-Match` igual responde 304. `Cache-Control: private, no-store` porque puede llevar códigos de reserva.
- **Modo sorpresa (NV-69):** la persona sorprendida recibe el mismo recorte que en `GET /planes/{id}`.
- **Límite:** política nueva `planes_paquete`, 30 por hora por usuario (`rate:planes_paquete:{usuario}`).

### Enlace compartido (corrección de drift)
- El Excel pide `GET /planes/compartidos/{codigo}`. NV-38 ya lo implementó como `POST /planes/compartidos` con `{codigo}` en el cuerpo, porque la ruta queda en los logs de acceso y de CloudFront.
- El código cumple lo que se pedía de un token firmado y más: 128 bits aleatorios imposibles de fabricar, solo su SHA-256 en la base, vence a los 7 días y se puede revocar (una firma HMAC sin estado no se puede revocar). La vista no lleva dueño, integrantes, correos, notas ni ids de personas.
- Esta spec solo agrega una prueba de seguridad: la vista pública de un plan `publico` sigue sin datos del dueño.

- **Variables:** ninguna nueva; radios y límites son constantes.

## Contratos
| Método | Ruta | Auth | Respuestas |
|---|---|---|---|
| GET | `/api/v1/planes/{id}/paradas/{parada_id}/alternativas?q=` | JWT dueño | 200, 401, 403, 404, 422, 429, 503 |
| POST | `/api/v1/planes/{id}/paradas/{parada_id}/cambiar` con `{negocio_id}` e `If-Match` | JWT dueño | 200, 400, 401, 403, 404, 409, 422, 429, 503 |
| GET | `/api/v1/planes/populares?lat=&lng=` | no | 200, 422, 429, 503 |
| GET | `/api/v1/planes/{id}/paquete-offline` | JWT dueño o integrante | 200, 304, 401, 404, 422, 429, 503 |
| PATCH | `/api/v1/planes/{id}` (gana `visibilidad`) | JWT dueño | como NV-30 |
| POST | `/api/v1/planes/{id}/clonar` (acepta planes `publico` ajenos) | JWT | como NV-30 |
| POST | `/api/v1/planes/compartidos` con `{codigo}` | no | sin cambios (NV-38) |

403 = cuenta en eliminación pendiente en las escrituras (`RequireActive`). `/planes/populares` se registra antes de `/planes/:id`. Al ejecutar, las filas pasan a `docs/endpoints.md` y las tres políticas a `docs/rate-limit-matrix.md`.

## Tests
- `tests/integration/planes_alternativas_test.go`: 3 parecidos abiertos y sin repetir los del plan, lista corta, `q`, cambiar recalcula costos y traslados y sube `version` y `sync_seq`, 409 con versión vieja, 400 sin `If-Match`, 422 con negocio cerrado o pausado, integrante y ajeno = 404.
- `tests/integration/planes_populares_test.go`: solo públicos de la semana dentro del radio, orden por `veces_clonado`, copiar un público ajeno suma 1 y crea uno `privado` con `plan_origen_id`, privado ajeno y bloqueado = 404, 429 por IP.
- `tests/integration/planes_paquete_test.go`: dueño e integrante, fuera de ventana = 422, 304 con `ETag`, ajeno = 404.
- `tests/security/planes_extras_security_test.go`: 401 en las rutas con sesión; populares y la vista de NV-38 sin `dueno_id`, nombre, correo ni integrantes; el punto redondeado no llega a los logs con más de 2 decimales; paquete con `no-store`.
- `tests/database/indexes_test.go`: `ix_populares` geográfico. `tests/contract/planes_extras_contract_test.go`: forma de las cuatro respuestas.
- `tests/integration/presupuesto_redis_test.go`: las tres políticas nuevas cuestan lo mismo que una ventana existente.
- Unit: filtros de alternativas, redondeo del punto, ventana del paquete y `ETag` (`internal/planes`).

## Criterios
- Ningún plan popular ni vista pública revela quién es su dueño, y cambiar una parada nunca pisa una edición más nueva.

## Límites
- Sin `GET /planes/compartidos/{codigo}` (sigue el `POST` de NV-38).
- Sin visibilidad `amigos` (el Excel la lista; queda para cuando haya feed de amigos).
- Sin imágenes dentro del paquete ni descarga programada en el servidor.
- Sin eventos en vivo del cambio de parada (los agrega la spec de tiempo real del grupo).

## Aprobación y Control de Cambios
- **Estado:** 🚧 Borrador (2026-11-13), ola por decidir
- **Revisores:** Osmar Reséndiz (pendiente de aprobar)
- **Fecha:** —
- **Decisiones para el revisor:** la vista compartida queda en el `POST` de NV-38 (código aleatorio con hash y revocable en lugar de token firmado); `visibilidad` solo `privado`/`publico`; `zona` = primera parada; punto redondeado a 2 decimales y caché HTTP pública de 5 min en vez de Redis; alternativas sin IA; versión en `If-Match`; paquete solo hoy o mañana.
- **Detalle:** evidencia en `specs/nv-68/checklists/gates.md` + `Memoria.md` al ejecutar.
