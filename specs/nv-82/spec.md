# NV-82 — Privacidad social

## Contexto
La maqueta (`28_PrivSocial`, `23_AmigosSolic`, `85_Ajustes`, `82_EditarPerfil`) pide controlar **quién ve mi actividad** (solo amigos, todos o nadie), **qué comparto** (reseñas en el feed, planes públicos, postales), si **aparezco en sugerencias** y **quién me puede mandar solicitudes** («Solo quien te agrega con tu usuario o código»). `privacidad` hoy solo tiene ubicación, aprendizaje, ánimo, datos sensibles, datos de uso, buscable por teléfono y por usuario. **Origen**: revisión de la maqueta (2026-11-21). **Depende de**: NV-05 (privacidad), NV-34/35/36 (búsqueda y amistades), NV-53 (sugerencias), NV-63 (momentos), NV-70 (perfil), NV-81 (pestañas).

## Requisitos
1. La persona elige quién ve su actividad y qué comparte; el valor por omisión es «solo amigos».
2. Puede salir de las sugerencias de amistad.
3. Puede limitar quién le manda solicitudes.
4. Cada módulo respeta estas opciones en todas sus rutas.

## Diseño
- **Campos nuevos en `privacidad.social`:** `actividad` (`amigos` por omisión, `todos`, `nadie`), `compartir_resenas` (true), `compartir_planes` (true: los marcados como públicos), `compartir_postales` (true), `en_sugerencias` (true), `solicitudes` (`cualquiera` por omisión o `codigo`). Ausentes en cuentas previas = valores por omisión.
- **Escritura:** `PATCH /usuarios/yo/privacidad` acepta `social{…}` parcial con la misma versión del perfil.
- **Efectos:**
  - **Actividad `nadie`:** perfil ajeno solo con datos básicos (nv-70), contadores `null`, pestañas vacías (nv-81), fuera del feed de momentos y de planes de amigos (nv-84). **`amigos`:** solo amigos ven pestañas y feed. **`todos`:** cualquier persona con sesión ve pestañas con contenido `publico`.
  - **`compartir_resenas=false`:** sus reseñas no salen en la pestaña ni en el feed (siguen en la ficha del lugar: son públicas y verificadas).
  - **`compartir_planes=false`:** ningún plan suyo en pestañas, feed ni populares.
  - **`compartir_postales=false`:** sus momentos solo los ve ella, aunque el momento diga otra visibilidad.
  - **`en_sugerencias=false`:** fuera de `GET /amistades/sugerencias` y de «Personas que quizá conozcas».
  - **`solicitudes=codigo`:** `POST /amistades` solo prospera si llega por `personas/codigo/{codigo}` (la app manda `origen: codigo`) o desde contactos con teléfono; desde búsqueda por nombre = 403 `solicitud_restringida`.
- **Exportación ARCO:** incluye `privacidad.social`.

## Contratos
| Método | Ruta | Cambio |
|---|---|---|
| PATCH | `/api/v1/usuarios/yo/privacidad` | acepta `social{actividad, compartir_resenas, compartir_planes, compartir_postales, en_sugerencias, solicitudes}` |
| GET | `/api/v1/usuarios/yo` | devuelve `privacidad.social` |
| POST | `/api/v1/amistades` | agrega `origen` (`busqueda`, `codigo`, `contactos`, `sugerencia`) con `codigo?` o `telefono?`; 403 `solicitud_restringida` |
| GET | `/api/v1/personas/{id}`, `/api/v1/personas/{id}/insignias` | con `actividad: nadie`, solo datos básicos e insignias vacías |
| GET | `/api/v1/momentos/feed`, `/api/v1/momentos/{id}`, `/api/v1/postales/{id}`, `/api/v1/lugares/{id}/postales` | respetan `actividad` y `compartir_postales` |
| GET | `/api/v1/planes/populares`, `/api/v1/lugares/{id}/amigos` | sin quien no comparte planes |
| GET | `/api/v1/amistades/sugerencias` | sin quien apagó `en_sugerencias` |

## Tests
- Unit: valores por omisión, validación de enums.
- Integration: cada opción contra perfil, pestañas, feed, sugerencias y solicitudes.
- Security: con `actividad: nadie` nada sale en ninguna ruta social (matriz por ruta); `solicitudes=codigo` no se salta con otro `origen` falso (el código se valida, no se confía en el campo).
- Contract: privacidad con `social`; exportación.

## Criterios
- Ninguna ruta social muestra actividad que la persona ocultó.
- Los valores por omisión no cambian lo que hoy se ve entre amigos.

## Límites
- Sin listas de amigos cercanos ni privacidad por contenido más allá de la visibilidad de cada momento.

## Drift al ejecutar (2026-11-21)
- **`amigos` y `todos` no cambian la visibilidad de cada momento:** hoy solo `nadie` y `compartir_postales=false` ocultan momentos y postales (feed, detalle, postal y galería del lugar). Un momento `publica` sigue público con `amigos`; la diferencia entre `amigos` y `todos` llega con las pestañas del perfil (nv-81). Porqué: «los valores por omisión no cambian lo que hoy se ve» y la galería pública del lugar se vaciaría.
- **Prueba de `origen`:** con `solicitudes=codigo`, `origen: codigo` exige `codigo` (el de amigo de esa persona) y `origen: contactos` exige `telefono` (su HMAC debe coincidir y la persona debe dejarse encontrar por teléfono). `busqueda`, `sugerencia` o sin origen = 403. El teléfono no se guarda ni se registra.
- **Código público propio:** `apperr.Error.WithCode` permite un `codigo` distinto al del tipo (403 `solicitud_restringida`) sin cambiar el estado HTTP.
- **Una solicitud recibida se acepta igual:** si la persona con `solicitudes=codigo` ya me envió una, reenviarla la acepta sin código (la relación existente se resuelve antes de la regla).
- **Rutas extra:** `GET /postales/{id}` (mismo efecto que el momento) y `GET /lugares/{id}/amigos` (sin quien no comparte planes) también lo respetan.
- **Pendiente en otras specs:** `compartir_resenas` solo se guarda (pestañas y feed de reseñas en nv-81); planes de amigos en nv-84; contadores `null` en nv-81.
- **Valores desconocidos en la base** caen al valor por omisión, nunca a uno más abierto.

## Aprobación y Control de Cambios
- **Estado:** 🚧 Borrador con evidencia (2026-11-21): ejecutada; ver `checklists/gates.md`.
- **Revisores:** Osmar Reséndiz (pendiente de aprobar)
- **Fecha:** —
- **Decisiones para el revisor:** (1) por omisión `amigos`; (2) las reseñas siguen públicas en la ficha del lugar aunque no se compartan en el feed; (3) `solicitudes=codigo` también permite contactos por teléfono.
- **Detalle:** `specs/nv-82/checklists/gates.md` al ejecutar.
