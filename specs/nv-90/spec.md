# NV-90 — Datos del negocio que pide la maqueta

## Contexto
El alta de negocio de la maqueta pide datos que hoy no se guardan: en `43_NegRegistro` el **tipo de experiencia** (Desayuno, Comida, Cena, Paseo, Romántico, En familia, Con amigos) y el **año de apertura**; en `44_NegUbic` el **WhatsApp del negocio** y **Instagram o sitio web**. El alta de nv-55 guarda dirección, ubicación, horarios, fotos, precio, servicios, dieta, alérgenos y accesibilidad. **Origen**: revisión de la maqueta (2026-11-21). **Depende de**: NV-55 (alta), NV-56 (perfil del negocio y cambios pendientes), NV-21 (ficha pública), NV-23/NV-31 (recomendador y planes con IA), NV-42 (moderación).

## Requisitos
1. El dueño captura tipo de experiencia, año de apertura, WhatsApp y una red o sitio web, en el alta y después en su perfil.
2. La ficha pública los muestra; el WhatsApp abre el chat con el negocio.
3. El tipo de experiencia ayuda a armar planes y a filtrar.

## Diseño
- **Campos en `negocios`:** `experiencias` (lista de 1 a 4 de `desayuno`, `comida`, `cena`, `paseo`, `romantico`, `familia`, `amigos`), `anio_apertura` (1900 al año actual), `whatsapp` (E.164 de México, normalizado como el teléfono de nv-35), `redes{instagram? (usuario 1–30 de `[A-Za-z0-9._]`, sin @), sitio_web? (https, ≤ 200, sin acortadores conocidos)}`.
- **Captura:** paso 2 del alta (`PATCH /negocios/{id}/alta/2`) acepta `whatsapp` y `redes`; el paso de datos del negocio (nv-55, `POST /negocios` o paso 1) acepta `experiencias` y `anio_apertura`; `PATCH /negocios/{id}` (nv-56) los acepta como cambios pendientes de revisión cuando el negocio ya está publicado (mismo criterio que `descripcion`).
- **Revisión:** el administrador ve los campos en el expediente (nv-55); `sitio_web` e `instagram` pasan por las reglas de moderación de texto (palabras prohibidas) y por una lista de dominios no permitidos.
- **Ficha pública (`GET /lugares/{id}`):** agrega `experiencias`, `anio_apertura`, `whatsapp_url` (`https://wa.me/52…`, sin exponer el número en otro campo) y `redes`.
- **Planes y filtros:** el recomendador usa `experiencias` como señal de compañía y momento del día (romántico ↔ pareja, familia ↔ familia, desayuno/comida/cena ↔ hora); `GET /lugares` acepta `experiencia=` (se suma a nv-85).
- **Privacidad:** son datos del negocio, públicos por decisión del dueño; el WhatsApp del negocio no es el teléfono personal del administrador (`42_NegCuenta`), que sigue privado.

## Contratos
| Método | Ruta | Cambio |
|---|---|---|
| POST | `/api/v1/negocios` | acepta `experiencias`, `anio_apertura` |
| PATCH | `/api/v1/negocios/{id}/alta/2` | acepta `whatsapp`, `redes` |
| PATCH | `/api/v1/negocios/{id}` | acepta los cuatro como cambios pendientes |
| GET | `/api/v1/lugares/{id}` | agrega `experiencias`, `anio_apertura`, `whatsapp_url`, `redes` |
| GET | `/api/v1/lugares` | acepta `experiencia=` |

## Tests
- Unit: validación de cada campo (año futuro, Instagram con @ o espacios, sitio http, acortador, WhatsApp sin lada).
- Integration: captura en el alta y como cambio pendiente; ficha pública; filtro `experiencia`; señal en el recomendador.
- Security: un comerciante no edita negocio ajeno; `sitio_web` con `javascript:` o `data:` = 422; el teléfono del administrador nunca sale en la ficha.
- Contract: ficha con los campos nuevos.

## Criterios
- La ficha muestra solo los datos que el dueño capturó y el administrador aprobó.

## Límites
- Sin varias redes por negocio (una de Instagram y un sitio); sin verificar que el WhatsApp sea del negocio.

## Drift al ejecutar (2026-11-23)
- **Paso 1 opcional:** `experiencias` y `anio_apertura` son opcionales en `POST /negocios` y no entran en los faltantes del envío a revisión; las altas viejas y las de la semilla no los tienen.
- **«Mismo criterio que `descripcion`»:** `descripcion` se aplica al momento (nv-56), no espera revisión. Se siguió la decisión (1) del revisor: los cuatro datos van a `cambios_pendientes` con el negocio publicado, como `nombre` y `direccion`; el administrador los ve en la cola (`estado=cambios_pendientes`) y en la fila de `en_revision`.
- **Quitar:** `whatsapp: ""` o `redes: {}` en el paso 2 o en el perfil quitan el dato (en el perfil, al aprobarse).
- **Palabras bloqueadas:** `moderacion.Service.Prohibido` usa solo la regla local de palabras de `moderacion_reglas` (sin llamar al modelo); si no se pueden leer las reglas, 503. Dominios no permitidos: lista fija de acortadores (`bit.ly`, `tinyurl.com`, `t.co`, `goo.gl`…) y sus subdominios; además se rechaza una URL con usuario (`https://a@b`).
- **Recomendador:** sin factor nuevo (los pesos de nv-23 no cambian). `experiencias` entra al factor `gustos`: la compañía llega como gusto (`recomendaciones` ya sumaba `romantico`/`familia`/`amigos`; el asistente ahora suma `ExperienciaDeCompania`) y la comida de la hora pedida (desayuno 6–11, comida 12–17, cena 18–23) suma una coincidencia. Sin gustos sigue neutro; un negocio sin `experiencias` puntúa igual que antes. Probado en Unit (`TestPuntaje_GustosConExperiencias`), no en Integration.
- **Índices:** ninguno; `experiencia=` se suma al `$geoNear`/filtros existentes. Sin variables nuevas.
- **Tests:** Unit `internal/negocios/datos_test.go`, `internal/negocios/alta/datos_test.go`, `internal/negocios/panel/datos_test.go`, `internal/recomendador/puntaje_test.go`; Integration `tests/integration/negocios_datos_test.go`; Security `tests/security/negocios_datos_security_test.go`; Contract `negocios/lugares_ficha_datos_200`.

## Aprobación y Control de Cambios
- **Estado:** 🚧 Borrador (2026-11-21), ejecutada el 2026-11-23 con evidencia en `checklists/gates.md`; falta la firma del revisor.
- **Revisores:** Osmar Reséndiz (pendiente de aprobar)
- **Fecha:** —
- **Decisiones para el revisor:** (1) los cuatro campos pasan por revisión cuando el negocio ya está publicado; (2) WhatsApp público solo como enlace `wa.me`; (3) `experiencias` como señal del recomendador.
- **Detalle:** `specs/nv-90/checklists/gates.md` al ejecutar.
