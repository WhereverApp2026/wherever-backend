# NV-16 — Versión de la API y versión mínima de la app

## Contexto
La app instalada debe saber si sigue vigente y la API debe poder pedir una actualización o anunciar mantenimiento sin dejar a nadie sin sesión. Hoy todas las rutas de negocio ya viven bajo `/api/v1` (nv-01) y `version_terminos` ya existe en la privacidad del usuario (nv-05), pero la app no tiene dónde consultar las versiones vigentes ni la API puede rechazar una versión vieja. **Origen**: TEC-29, TEC-30, US-001, US-003. **Depende de**: NV-05 (`privacidad.version_terminos`) y NV-11 (valores por entorno en Parameter Store).

## Requisitos
1. **Versionado de la API (TEC-29):** toda ruta de negocio vive bajo `/api/v1`; una ruta de negocio sin versión da 404. `/health` y `/ready` quedan fuera de `/api/v1`, igual que `/docs` y `/openapi.json` (solo en dev). Un cambio incompatible crea `/api/v2` sin retirar `/api/v1`; esta spec no crea `v2`, solo verifica la regla.
2. **Configuración pública (US-001):** `GET /api/v1/configuracion`, sin autenticación, devuelve exactamente `version_minima_app`, `version_actual_app`, `version_terminos`, `version_aviso_privacidad` y `mantenimiento`. Los valores salen de la configuración del proceso, no de una base.
3. **Versión mínima (TEC-30):** la app manda `X-App-Version` con un semver numérico (`X.Y.Z`; se admite una `v` inicial). La comparación es numérica por componente. Una cabecera ausente, vacía o ilegible se **ignora** y la petición sigue (nunca 400). Una versión válida e inferior a `version_minima_app` responde **426**. El 426 y el 503 de mantenimiento solo aplican a rutas bajo `/api/v1/`; cualquier otra ruta sigue al enrutador y conserva su 404. Igual o superior sigue normal. Exentas del 426: `/health`, `/ready`, `/api/v1/ping`, `/api/v1/configuracion`, `/api/v1/auth/*` y, solo en dev, `/docs` y `/openapi.json`. Auth queda exenta para que una app vieja pueda cerrar sesión, refrescar y recuperar la cuenta mientras se actualiza.
4. **Mantenimiento:** con `MODO_MANTENIMIENTO=true`, la configuración publica `mantenimiento: true` y toda ruta no exenta responde **503** con el cuerpo uniforme y `Retry-After: 60`. Las rutas exentas conservan su respuesta para que el balanceador, la app y el cierre de sesión sigan funcionando. El valor se lee al arrancar; cambiarlo exige reescribir el `.env` y reiniciar la API en las dos instancias. El actualizador no lo hace: solo reinstala cuando hay una imagen nueva.
5. **Condicional (US-001):** la respuesta de configuración lleva un `ETag` débil y `Cache-Control: no-cache`. Con `If-None-Match` vigente responde **304** sin cuerpo. Es el único endpoint con ETag en esta spec; nv-17 generaliza la técnica a los catálogos públicos.

## Diseño
- **Paquetes:**
  - `internal/platform/versionado`: tipo `Version` (`Parse`, `String`, `MenorQue`) y `Politica` (mínima, mantenimiento y lista de rutas exentas) con `Decidir(ruta, cabecera)`; sin dependencias internas.
  - `internal/http/middleware/version.go`: etapa `version`; traduce la decisión a `426` o `503` sin conocer Fiber en el paquete de dominio.
  - `internal/shared/etag`: `De(bytes)` (ETag débil) y `Coincide(ifNoneMatch, etag)`; sin Fiber para poder probarlo aparte y reutilizarlo en nv-17.
  - `internal/configuracion`: `handler.go`, `service.go` y `dto.go`. No tiene repositorio: el service recibe `config.VersionApp` por constructor y devuelve una copia inmutable.
- **Pipeline:** la etapa `version` entra **después de `rate_limit_writes` y antes de `jwt`**. Solo actúa sobre rutas bajo `/api/v1/` no exentas: corre antes del enrutador y, si actuara sobre todo, una ruta inexistente daría 426 en vez de 404. Ya corrieron headers de seguridad, CORS y el límite; no se consulta Redis de revocación ni se valida un token para responder 426/503. Una app vieja consume cuota, pero la petición es barata y el limitador evita el abuso. Si hay mantenimiento, el 503 manda sobre el 426 (la app ya ve ambos valores en la configuración).
- **Enmienda de la constitución:** agregar la etapa a §4, el código `426` a la lista de §4 y `UpgradeRequired→426` a §5, y dejar la lista del pipeline igual al código real (hoy omite `request_timeout` y la separación de los dos límites). Sube a la versión 1.7.0 (la 1.6.0 ya la usó nv-12). En el mismo PR se actualizan `scripts/critic-guardrails.sh` (B3) y `TestPipeline_ConstitutionalOrder`; sin la enmienda aprobada, nv-16 no se fusiona.
- **Configuración:** claves nuevas en `internal/platform/config`, todas opcionales con defecto y validadas al arrancar:

| Clave | Campo | Defecto | Validación |
|---|---|---|---|
| `APP_VERSION_MINIMA` | `VersionApp.Minima` | `0.0.0` | semver numérico y ≤ `version_actual_app` |
| `APP_VERSION_ACTUAL` | `VersionApp.Actual` | `1.0.0` | semver numérico y ≥ mínima |
| `TERMINOS_VERSION` | `VersionApp.Terminos` | `2026-10` | 1 a 20 caracteres, sin espacios |
| `AVISO_VERSION` | `VersionApp.Aviso` | `2026-11` | 1 a 20 caracteres, sin espacios; sube cuando cambia `docs/aviso-privacidad.md` (2026-11 agregó el proveedor de IA, nv-26) |
| `MODO_MANTENIMIENTO` | `VersionApp.Mantenimiento` | `false` | `true` o `false`, sin distinguir mayúsculas; otro valor falla al arrancar |

  `2026-10` es la versión que hoy manda la app al registrarse; cambiarla es una decisión de producto que ejecuta nv-19. En el servidor las cinco claves viven en `/wherever/dev/` y las toma `deploy-env.sh` como las demás.
- **Encender o apagar el mantenimiento en el servidor:** cambiar `MODO_MANTENIMIENTO` en Parameter Store y, por Systems Manager en cada instancia, correr `deploy-env.sh` y `docker compose up -d`. Primero una instancia y luego la otra: mientras tanto conviven las dos respuestas. El procedimiento va en `infra/aws-dev/README.md`.
- **Respuesta:**
  ```json
  {
    "version_minima_app": "1.4.0",
    "version_actual_app": "1.6.2",
    "version_terminos": "2026-10",
    "version_aviso_privacidad": "2026-11",
    "mantenimiento": false
  }
  ```
  `version_actual_app` es la última versión publicada en las tiendas y es informativa; la versión de la API sigue saliendo en `/health`.
- **Errores:** se agrega `apperr.KindUpgradeRequired` (426, código `actualizacion_requerida`, mensaje «actualiza la app a la versión X o superior») al final de la lista de tipos para no mover los valores existentes. El 503 de mantenimiento reutiliza `apperr.Unavailable("en mantenimiento")` con `RetryAfter: 60`; no se crea otro tipo con estado 503 porque `KindFromStatus` recorre un mapa y el resultado no sería determinista.
- **ETag:** una sola serialización a JSON; el ETag se calcula sobre esos mismos bytes (`W/"` + primeros 32 hex de SHA-256 + `"`). Es débil porque la etapa `compress` puede mandar el cuerpo en gzip y un ETag fuerte debe cambiar con la codificación (RFC 9110); para el 304 basta la comparación débil. `If-None-Match` acepta el valor con o sin `W/`, una lista o `*`. La respuesta no varía por `X-App-Version`; el único `Vary` es el `Accept-Encoding` que ya pone la compresión.
- **CORS:** `X-App-Version` e `If-None-Match` se agregan a los encabezados permitidos, y `ETag` a los expuestos. Sin eso, un cliente de navegador no puede mandar la versión ni aprovechar el 304 Hoy tampoco se expone el `ETag` del perfil de nv-05, que se usa con `If-Match` (ese sí está permitido).
- **Datos:** sin colecciones ni índices nuevos. `usuarios.privacidad.version_terminos` no cambia en esta spec.

## Contratos
| Método | Ruta | Auth | Rate | Respuestas |
|---|---|---|---|---|
| GET | `/api/v1/configuracion` | no | global | 200, 304 |

Desde nv-16 toda ruta no exenta puede responder además **426** (cabecera válida y menor) o **503** (mantenimiento); no se repite en cada fila de `docs/endpoints.md`, que gana una nota. Los códigos propios de cada ruta no cambian. La cabecera de petición es `X-App-Version: X.Y.Z`; la de respuesta, `ETag`.

**426** (cabecera válida y menor):
```json
{"codigo":"actualizacion_requerida","mensaje":"actualiza la app a la versión 1.4.0 o superior","campos":[],"traza_id":"…"}
```
**503** de mantenimiento, con `Retry-After: 60`:
```json
{"codigo":"no_disponible","mensaje":"en mantenimiento","campos":[],"traza_id":"…"}
```

## Tests
- `Unit/Versionado 12`: `1.2.3` válido, `v1.2.3` válido, `1.2`, `1.2.3.4`, `1.2.x` y vacío inválidos; `1.10.0` mayor que `1.9.0`; iguales; menor; basura ignorada; rutas exentas; el mantenimiento manda sobre el 426.
- `Unit/Configuración 6`: defectos, valores explícitos, semver inválido falla el arranque, mínima mayor que actual falla, `MODO_MANTENIMIENTO` inválido falla, versión de términos vacía usa el defecto.
- `Unit/ConfiguraciónHTTP 4`: 200 con los cinco campos y sus tipos, ETag estable, 304 con `If-None-Match` y `mantenimiento: true` en la respuesta.
- `Unit/MiddlewareVersión 9` con `app.Test`: versión vieja → 426 en una ruta de negocio, igual y nueva → 200, ausente → 200, ilegible → 200, `/api/v1/auth/login` exenta → 200, mantenimiento → 503 con `Retry-After: 60`, configuración en mantenimiento → 200, ruta fuera de `/api/v1` con versión vieja → 404.
- `Unit/ETag 5`: ETag débil y estable para el mismo cuerpo, distinto al cambiar un valor, `If-None-Match` con lista, con `*` y sin el prefijo `W/`.
- `Unit/CORS 2`: el preflight permite `X-App-Version` e `If-None-Match` y la respuesta expone `ETag`.
- `Contract/Configuración 2`: forma exacta de la respuesta y variante con `mantenimiento: true` (`tests/contract/testdata/configuracion/`).

## Criterios
- Una app vieja recibe 426 en las rutas de negocio y sigue pudiendo leer la configuración, cerrar sesión, refrescar y recuperar la cuenta.
- La configuración responde 200 o 304, también en mantenimiento, y su ETag cambia cuando cambia cualquier valor.
- `1.10.0` se compara como mayor que `1.9.0`; nunca se compara texto.
- Una ruta sin `/api/v1` sigue dando 404.
- La respuesta y los logs no llevan secretos, correos ni datos personales.

## Límites
- El 426 es un aviso de compatibilidad, no un control de acceso: un cliente puede omitir la cabecera. La app debe mandarla.
- Solo se admiten versiones numéricas `X.Y.Z` con `v` opcional; una pre-release o un `+build` se trata como cabecera ausente y no bloquea (los canales internos de las tiendas no se bloquean).
- `APP_VERSION_MINIMA` sin definir vale `0.0.0` y no bloquea a nadie; en el servidor el operador debe fijarla en Parameter Store para que la función tenga efecto.
- `version_actual_app` no se consulta a las tiendas; la mantiene el operador.
- `MODO_MANTENIMIENTO` se lee al arrancar y no se puede cambiar en caliente (la configuración es inmutable por proceso); en el servidor hace falta el paso manual de Diseño.
- El ETag se implementa solo para `/api/v1/configuracion`; nv-17 lo generaliza a los catálogos públicos. La caché de Redis es de nv-18 y aquí no se usa.
- nv-16 publica las versiones de términos y aviso, pero **no** exige su aceptación: eso es nv-19.
- Sin `/api/v2`: esta spec no crea una segunda versión, solo deja la regla verificada.
- Sin la enmienda de la constitución §4/§5 aprobada (1.7.0), la etapa y el 426 no se implementan.
- No hay entorno de staging: las pruebas finales se hacen en el servidor de desarrollo, y el modo mantenimiento corta a quien lo esté usando en ese momento.

## Aprobación y Control de Cambios
- **Estado:** 🚧 Borrador revisado (2026-11-09); ola 4: plataforma y catálogo completo, después de la ruta prioritaria (ver `Memoria.md`)
- **Revisores:** Osmar Reséndiz (pendiente de aprobar)
- **Fecha:** —
- **Decisiones para el revisor antes de ejecutar:**
  1. Enmienda de la constitución §4/§5 (1.7.0) para agregar la etapa `version` (entre los límites y JWT, solo sobre `/api/v1/`) y el código `426`; en el mismo PR se actualizan `critic B3` y `TestPipeline`. La lista de §4 se deja igual al pipeline real, incluido `request_timeout`.
  2. `X-App-Version` ausente o inválida no bloquea; solo un semver válido y menor da 426. Es aviso de compatibilidad, no autorización.
  3. Rutas exentas: `/health`, `/ready`, `/api/v1/ping`, `/api/v1/configuracion`, `/api/v1/auth/*` y, en dev, `/docs` y `/openapi.json`.
  4. `MODO_MANTENIMIENTO` se aplica en el servidor (503) y se anuncia en la configuración; se lee al arrancar y en el servidor se cambia con el paso manual por Systems Manager.
  5. Valores por defecto: `APP_VERSION_MINIMA=0.0.0`, `APP_VERSION_ACTUAL=1.0.0`, `TERMINOS_VERSION=2026-10`, `AVISO_VERSION=2026-11`, `MODO_MANTENIMIENTO=false`; en el servidor van en Parameter Store.
  6. ETag débil y 304 solo para `/api/v1/configuracion`; nv-17 lo generaliza.
- **Detalle:** la evidencia irá en `specs/nv-16/checklists/gates.md` + `Memoria.md`.
