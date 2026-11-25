# NV-92 — Corazón de favorito en las tarjetas

## 1. Objetivo
Las tarjetas de lugares no dicen si la persona ya guardó el lugar: la app tiene que pedir aparte `GET /favoritos/ids?tipo=negocio` (nv-54) y cruzar los ids para pintar el corazón. Esta spec agrega `es_favorito` (booleano) a las tarjetas cuando hay sesión. **Origen**: revisión de lo que pide la app (2026-11-25). **Depende de**: NV-21 (`GET /lugares`), NV-24 (`GET /recomendaciones`), NV-25 (`GET /inicio`), NV-54 (favoritos), NV-85 (tarjeta de Explorar).

## 2. Alcance / No-alcance
- **Dentro:** `es_favorito` en la tarjeta de `GET /inicio` (`recomendaciones[]`), `GET /recomendaciones` (`items[]`) y `GET /lugares` (`items[]`, la tarjeta de Explorar). Una consulta a `favoritos` por respuesta.
- **Fuera (decisión D-1):**
  - `GET /lugares/mapa`: devuelve pines y celdas (`id`, `nombre`, `categoria`, `ubicacion`), no tarjetas; el pin abre la ficha. Sumarlo costaría una consulta en la ruta más llamada al mover el mapa.
  - `GET /planes/populares`: son planes, no lugares, y la respuesta es pública con `Cache-Control: public, max-age=300` (sin sesión). Marcarla rompería esa caché.
  - Las demás listas que reutilizan `PlaceItem` (búsquedas, recientes, grupos, planes) siguen igual: no son tarjetas con corazón en la maqueta.
  - `GET /favoritos/ids` se queda (lo usan otras pantallas y los planes).

## 3. Contratos API
| Método | Ruta | Cambio |
|---|---|---|
| GET | `/api/v1/inicio` | cada tarjeta de `recomendaciones[]` suma `es_favorito?` |
| GET | `/api/v1/recomendaciones` | cada tarjeta de `items[]` suma `es_favorito` |
| GET | `/api/v1/lugares` | cada tarjeta de `items[]` suma `es_favorito?` |

- **Con sesión:** `es_favorito` siempre va, `true` o `false`, según los favoritos de tipo `negocio` de quien consulta.
- **Sin sesión: el campo se omite** (decisión D-2). Porqué: `false` diría «no lo guardaste» cuando no se sabe quién pregunta; omitirlo sigue la regla de `amigos_fueron` (nv-85) y la app solo pinta el corazón si el campo existe.
- `GET /inicio` con la cuenta no activa sigue como sin sesión (nv-25): recomendaciones generales y sin `es_favorito`.
- Sin rutas nuevas ni códigos nuevos. Si la lectura de favoritos falla, la respuesta es 503 como cualquier otra lectura de la lista.

## 4. Modelo de datos
- Sin colecciones, campos ni índices nuevos. La consulta es `favoritos` con `{usuario_id, tipo: "negocio", objetivo_id: {$in: ids de la página}}` y proyección de `objetivo_id`; la cubre el prefijo de `ux_favorito` (`usuario_id`, `tipo`, `objetivo_id`).
- Una sola consulta por respuesta, con los ids de la página (≤ 50 en `/recomendaciones` y `/lugares`, 5 en `/inicio`): sin N+1.

## 5. Seguridad
- `es_favorito` sale solo de los favoritos del dueño del token: nunca de los de otra persona. Los favoritos son privados (nv-54) y el campo no dice cuántas personas guardaron el lugar.
- Sin sesión no se consulta `favoritos` ni aparece el campo. Token inválido sigue en 401 (nv-01).
- `GET /recomendaciones` y `GET /inicio` mantienen `Cache-Control: no-store`; `GET /lugares` no se cachea por persona.

## 6. Tests / Gates
- **Unit:** `internal/negocios/favoritos_test.go` (el armado: con marcas va `true`/`false` por tarjeta; sin sesión o sin dependencia = nil y el JSON omite `es_favorito`).
- **Integration:** `tests/integration/favoritos_tarjetas_test.go`: con un lugar en favoritos y otro no, `GET /lugares`, `GET /recomendaciones` y `GET /inicio` devuelven `true` y `false`; sin sesión el campo no aparece; otra persona ve `false` en el lugar que marcó la primera.
- **Contract:** `negocios/lugares_tarjeta_200` (tarjeta de Explorar con sesión) recapturado con `CONTRACT_CAPTURE=1`: suma `es_favorito: bool`. `GET /recomendaciones` e `GET /inicio` no tienen captura de contrato; su forma la cubre Integration.
- **Database / Security:** sin índices ni rutas nuevas; la autorización ya la cubren las suites de nv-21, nv-24 y nv-25.

## 7. Plan de entrega
- Sin índices ni variables nuevas. Se despliega con la imagen; la app puede dejar de cruzar `GET /favoritos/ids` en Inicio y Explorar.

## Aprobación
- **Estado:** 🚧 Borrador (2026-11-25), ejecutada el mismo día; falta la firma del revisor.
- **Revisores:** Osmar Reséndiz (pendiente de aprobar)
- **Decisiones para el revisor:** (1) D-1, mapa y planes populares fuera; (2) D-2, sin sesión el campo se omite.
- [ ] Revisor 1 — fecha:
- [ ] CHECKLIST_PR.md OK
- [x] Sin conflicto con constitución
