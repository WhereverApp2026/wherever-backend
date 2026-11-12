# NV-17 — Paginación por cursor y validadores HTTP

## Contexto
La paginación por cursor de nv-02 codifica `{"_id","orden"}` en Base64 URL sin firma: un cliente puede alterar el cursor y elegir otro punto de inicio. El único `ETag` de negocio es el del perfil (`/usuarios/yo`, nv-05), que se usa con `If-Match`; nv-16 aporta `internal/shared/etag` y el 304 de `GET /api/v1/configuracion`, y nv-17 generaliza la técnica a los catálogos públicos y declara `no-store` en los datos del usuario. **Origen**: TEC-29, TEC-30, US-001, US-003. **Depende de**: NV-16 (`internal/shared/etag`, CORS con `If-None-Match` y `ETag`).

## Requisitos
1. **Cursor firmado (TEC-29, US-003):** toda lista usa `?limite=` (20 por defecto, 50 máximo) y `?cursor=` opaco. El cursor guarda `_id`, `orden` y `ambito`, va en Base64 URL sin relleno y lleva un HMAC-SHA-256 con `CURSOR_HMAC_KEY`; la firma cubre el JSON completo. Un cursor alterado, firmado con otra llave, de otra ruta/ámbito o con un `_id` que no es ObjectId da **422** con `campos: [{campo: "cursor"}]`, nunca 500. Cursor vacío = primera página. `PageResult{items, siguiente_cursor, limite}` no cambia.
2. **Verificación en el service:** el validador del DTO solo comprueba la forma (Base64 URL sin relleno, JSON estricto y `_id` ObjectId). `PaginationQuery.Position(signer, scope)` verifica la firma y el ámbito, y traduce `ErrInvalidCursor` al **422** con `campo: "cursor"`. Ningún service interpreta `q.Cursor` por su cuenta; no hay estado global.
3. **ETag/304 en catálogos públicos:** todo GET público de catálogo usa `internal/shared/etag`. Se aplica a `GET /api/v1/ciudades/autocompletar`, que responde `ETag` débil y `Cache-Control: public, max-age=60`. Con `If-None-Match` vigente responde **304** sin cuerpo; si el catálogo cambia, 200 con ETag nuevo.
4. **Datos del usuario:** `GET /api/v1/usuarios/yo` y `GET /api/v1/auth/sesiones` llevan `Cache-Control: no-store`. El perfil conserva su `ETag` para `If-Match` (nv-05) pero no responde 304; sesiones tampoco.
5. **Contrato de llave:** el cursor se firma con `CURSOR_HMAC_KEY` (32 bytes o más), una llave dedicada distinta de `JWT_KEY` y `PHONE_HMAC_KEY`. Ya hay dos consumidores (`GET /lugares` de nv-21 y `GET /planes` de nv-30), así que la clave entra en nv-17 en `internal/platform/config` (obligatoria, igual que `PHONE_HMAC_KEY`), `.env.example`, `deploy-env.sh` y `/wherever/dev/` (la carga el operador). `NewSigner` rechaza llaves cortas. Rotarla invalida los cursores activos: el cliente recibe 422 y reinicia en la primera página.

## Diseño
- **Paquete:** se reutiliza `internal/shared/pagination`; no se crea otro. `cursor.go` gana `Signer` y `page_result.go` no cambia. `internal/shared/etag` viene de nv-16 y aquí solo se consume.
- **Formato del cursor:** `base64url(JSON{_id,orden,ambito}) + "." + base64url(HMAC-SHA-256(llave, JSON))`. Al decodificar se verifica el MAC con `hmac.Equal` antes de interpretar el JSON; luego se exige JSON estricto (sin campos extra), `_id` ObjectId y `ambito` no vacío. Vacío = primera página. `Decode(scope, raw)` además exige `ambito == scope`; un cursor de otra ruta, filtro o usuario no se acepta.
- **Ámbito:** lo elige cada lista. Si la lista tiene filtros o es del usuario, el ámbito incluye un hash estable de los filtros o el id del usuario (`"lugares:"+hash(filtros)`, `"reservas:"+usuario`); así un cursor de otra búsqueda o de otra cuenta da 422. El token es firmado, no cifrado: no se pone correo, nombre ni otro dato personal en `ambito` ni en `orden`.
- **Límites:** se conservan 20/50 y `NewPaginationQuery` con 20 por defecto. `limite=0`, negativo o 51 sigue dando 422; no se agrega otra política.
- **Validación:** `pagination.Valid(raw)` comprueba solo la forma; es lo que usa la regla `cursor` de `validation.New()`, que no cambia de firma. `Signer.Decode(scope, raw)` verifica MAC y ámbito y es el único camino para obtener un `Cursor`; `PaginationQuery.Position(signer, scope)` convierte `ErrInvalidCursor` en `apperr.Validation` con el campo `cursor`.
- **Llave y configuración:** `CURSOR_HMAC_KEY` (32+ bytes, dedicada, sin el marcador de ejemplo) es obligatoria en `internal/platform/config`. `cmd/api` arma un solo `Signer` en `buildBusinesses` y lo inyecta con `WithCursor` en los services de negocios y planes; `authtest` hace lo mismo con `testutil.CursorKey`. `NewSigner` valida el tamaño; un service sin `Signer` falla cerrado.
- **Ámbito real:** `Signer.Scope(nombre, partes…)` da `nombre:` + HMAC truncado de las partes, con llave, para que la ubicación o el id no se lean del cursor. `GET /lugares`: orden, categoría, precio máximo, abierto ahora, radio y, si hay, lat/lng. `GET /planes`: id del dueño y pestaña. `validation.Position(signer, scope, raw)` sirve a las listas con DTO propio.
- **Tamaño:** el cursor firmado mide hasta ~220 caracteres; `cursor` en los DTO de lugares y planes sube de `max=200` a `max=300` y lugares gana la regla de forma `cursor`.
- **ETag:** una sola serialización a JSON; el ETag se calcula sobre esos bytes con el ayudante de nv-16 (`W/"…"`). `If-None-Match` acepta `W/`, listas y `*`. El 304 se responde con `c.Status(304); return nil` para no escribir cuerpo; `ETag` y `Cache-Control` se fijan antes. Se aplica a `ciudades/autocompletar` con `Cache-Control: public, max-age=60`. En dev CloudFront tiene la caché desactivada (nv-12), así que el `max-age` vale sobre todo para el cliente; el ETag evita reenviar el cuerpo.
- **No-store:** `sendProfile` (`/usuarios/yo` y sus cambios) y `listSessions` (`/auth/sesiones`) fijan `Cache-Control: no-store`. El perfil mantiene el ETag de versión para `If-Match`; el servidor no convierte su `If-None-Match` en 304.
- **Documentación:** `docs/endpoints.md` gana una nota de paginación firmada y las cabeceras de las tres rutas afectadas; `internal/http/openapi.json` documenta el 304 de autocompletar y el `no-store` de las privadas.
- **Datos:** sin colecciones, índices ni claves de Redis nuevos. El 304 no evita la consulta a Mongo; la caché es de nv-18.

## Contratos
Sin endpoints nuevos. Cambian cabeceras y un código en rutas existentes:

| Método | Ruta | Auth | Rate | Respuestas | Cambio |
|---|---|---|---|---|---|
| GET | `/api/v1/ciudades/autocompletar?q=&limite=` | no | global | 200, **304**, 400, 422 | `ETag` y `Cache-Control: public, max-age=60`; 304 con `If-None-Match` |
| GET | `/api/v1/usuarios/yo` | JWT | global | 200, 401 | `ETag` (ya existía) y `Cache-Control: no-store`; sin 304 |
| GET | `/api/v1/auth/sesiones` | JWT | global | 200, 401 | `Cache-Control: no-store`; sin 304 |

`GET /api/v1/lugares` y `GET /api/v1/planes` pasan a `?cursor=` firmado: sus cursores sin firma (emitidos antes de nv-17) dan 422 y el cliente reinicia en la primera página. Las listas nuevas usan el mismo molde. `siguiente_cursor` vacío sigue significando fin de lista.

**422 de cursor** (la forma la detecta el validador; la firma y el ámbito los detecta `Position`):
```json
{"codigo":"validacion","mensaje":"datos inválidos","campos":[{"campo":"cursor","mensaje":"cursor inválido"}],"traza_id":"…"}
```
**304 de autocompletar:** sin cuerpo; conserva `ETag` y `Cache-Control`.

## Tests
- `Unit/Cursor 13`: ida y vuelta con ámbito; vacío = primera página; carga alterada; MAC alterado; otra llave; otro ámbito; llave corta; Base64 inválido; Base64 con relleno; JSON roto; campo extra; `_id` que no es ObjectId; ámbito vacío al codificar.
- `Unit/Listas`: lugares (ida y vuelta por orden, otro orden, otra búsqueda, ámbito sigue a los filtros sin exponer la ubicación) y planes (otra pestaña, otra cuenta).
- `Integration/Lugares`: cursor de otro orden, de otra búsqueda, de otra llave y alterado = 422.
- `Unit/Validación 4`: `PaginationQuery` conserva 20/50 y rechaza 0/51/negativo; un cursor firmado válido pasa la forma; `Position` con cursor alterado da 422 con `cursor`; `Position` con ámbito ajeno da 422 con `cursor`.
- `Unit/Fuzz 1`: `FuzzDecodeCursor` no entra en pánico con ninguna entrada; un cursor aceptado se vuelve a codificar y decodificar igual.
- `Integration/ETag 4` con Mongo real: autocompletar 200 con ETag y `public, max-age=60`; el mismo `If-None-Match` da 304 sin cuerpo; al cambiar el catálogo, 200 con ETag nuevo; `/usuarios/yo` y `/auth/sesiones` llevan `no-store`, y el perfil con `If-None-Match` sigue en 200, no 304.
- `Contract`: sin cambios; no hay cuerpos nuevos y el 304 no lleva cuerpo.

## Criterios
- Un cursor alterado, de otra llave o de otro ámbito da 422 con `campo: cursor`; nunca 500 ni una página de otro usuario.
- El mismo cursor reanuda en el mismo punto; el desempate por `_id` mantiene el orden estable.
- Un catálogo público devuelve 304 cuando no cambió y 200 con ETag nuevo cuando cambió.
- Un GET con datos del usuario nunca devuelve 304 y siempre lleva `no-store`.
- Toda lista nueva posterior a nv-17 usa `PaginationQuery`, `PageResult` y el `Signer`; no queda una variante sin firmar.
- `go build ./...` y `go vet ./...` quedan verdes; `validation.New` conserva su firma y el validador no conoce la llave.

## Límites
- Corrección de drift (2026-11-12): la spec suponía que no había lista que migrar, pero nv-21 (`/lugares`) y nv-30 (`/planes`) ya paginaban con el cursor sin firmar. Se migran aquí y `CURSOR_HMAC_KEY` entra ya en config. `GET /auth/sesiones` no se pagina porque son pocas y no es un catálogo.
- El catálogo de categorías de nv-20 sigue sin ETag; se suma al molde cuando nv-18 lo cachee.
- El cursor está firmado, no cifrado: su contenido se puede leer. No se pone información personal en `orden` ni en `ambito`.
- Rotar `CURSOR_HMAC_KEY` invalida todos los cursores activos; el cliente recibe 422 y reinicia en la primera página. Es aceptable y no requiere migración de datos.
- Antes de desplegar, el operador carga `/wherever/dev/CURSOR_HMAC_KEY` (SecureString, 32+ bytes aleatorios); sin ella `deploy-env.sh` se detiene y la API no arranca.
- El 304 ahorra el cuerpo, no la consulta a Mongo; la caché de Redis es de nv-18. CloudFront de desarrollo no cachea.
- `public, max-age=60` es una propuesta de frescura; si el producto la quiere más corta, se ajusta sin cambiar el ETag.
- El ETag de `/usuarios/yo` sigue siendo para `If-Match` (nv-05); esta spec no lo convierte en 304 ni cambia la concurrencia.
- nv-17 depende de nv-16: sin `internal/shared/etag` y sin `If-None-Match`/`ETag` en CORS, no se ejecuta.
- Sin endpoints nuevos, sin cambios de base ni de índices, sin Redis y sin tocar el orden del pipeline (`nightly.yml` solo suma la variable al contenedor de ZAP).
- La firma del cursor sustituye la decisión de nv-02 (cursor sin firmar): aunque el service filtre por dueño, la integridad y el ámbito evitan páginas ambiguas y preparan las listas con filtros posteriores.

## Aprobación y Control de Cambios
- **Estado:** ✅ Aprobada e implementada (2026-11-12), con la corrección de drift: migra `/lugares` y `/planes` y `CURSOR_HMAC_KEY` entra en config.
- **Revisores:** Osmar Reséndiz (aprobó las seis decisiones con el ajuste, 2026-11-12)
- **Fecha:** 2026-11-12
- **Decisiones para el revisor antes de ejecutar:**
  1. Firmar el cursor con HMAC-SHA-256 y una llave dedicada `CURSOR_HMAC_KEY` (32+ bytes), no con `JWT_KEY` ni `PHONE_HMAC_KEY`. Ajuste aprobado: entra ya en config porque `/lugares` y `/planes` la consumen.
  2. Incluir `ambito` en el JSON firmado y comprobarlo en el service; si la lista tiene filtros o es del usuario, el ámbito lleva un hash de los filtros o el id del usuario.
  3. No cambiar `validation.New`: la regla `cursor` sigue comprobando solo la forma; `PaginationQuery.Position(signer, scope)` verifica firma y ámbito. El validador no conoce la llave y no hay estado global.
  4. ETag/304 solo para el catálogo público (`ciudades/autocompletar`) con `public, max-age=60`; `no-store` en `/usuarios/yo` y `/auth/sesiones`, sin 304 en privadas.
  5. No hay endpoints nuevos; se migran `/lugares` y `/planes` (ajuste aprobado). Sesiones no se pagina.
  6. El operador carga `/wherever/dev/CURSOR_HMAC_KEY` antes de desplegar (ajuste aprobado).
- **Detalle:** la evidencia irá en `specs/nv-17/checklists/gates.md` + `Memoria.md`.
