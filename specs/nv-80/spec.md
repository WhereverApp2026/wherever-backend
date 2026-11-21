# NV-80 — Regla y vigencia del Sello de Excelencia

## Contexto
La maqueta (`34_Sello`, `51_NegResenas`, `52_NegResp`) dice que el Sello se gana con **4.5 o más**, **50 reseñas verificadas** y **responder al menos 9 de cada 10 reseñas**, y que es **vigente hasta** una fecha («Vigente hasta marzo de 2027»). La regla de nv-56 pide ≥ 60 días aprobado, ≥ 20 reseñas verificadas en 90 días con promedio ≥ 4.5 y ningún rechazo de moderación; no cuenta respuestas y el Sello no vence. **Origen**: revisión de la maqueta (2026-11-21). **Depende de**: NV-56 (Sello y avance), NV-62 (reseñas y respuestas), NV-76 (insignia), NV-40 (avisos).

## Requisitos
1. El avance del Sello que ve el dueño usa los criterios de la maqueta, incluida la tasa de respuesta.
2. El Sello tiene vigencia y vence si no se renueva; el dueño ve hasta cuándo.
3. La app puede mostrar los criterios del Sello sin escribirlos a mano.

## Diseño
- **Criterios (tabla en `internal/negocios/panel/sello.go`):** promedio ≥ 4.5 de reseñas verificadas de los últimos 12 meses; ≥ 50 reseñas verificadas en 12 meses; tasa de respuesta ≥ 90 % en los últimos 90 días (reseñas con respuesta publicada / reseñas publicadas, con al menos 10 reseñas en el periodo); ningún contenido del negocio rechazado por moderación en 90 días (se conserva de nv-56); ≥ 60 días aprobado (se conserva).
- **Avance:** `GET /negocios/{id}/panel` y `GET /negocios/{id}/resenas/resumen` agregan `tasa_respuesta` y los criterios nuevos en `avance_sello`.
- **Vigencia:** `PUT /admin/negocios/{id}/sello` con `otorgar: true` fija `sello.vigente_hasta` = otorgado + 6 meses (configurable `SELLO_VIGENCIA_MESES`, 1–24). El trabajo nocturno de nv-56 revisa los Sellos que vencen: si el negocio sigue cumpliendo, avisa al administrador (cola) para renovar; si vence sin renovar, `sello_excelencia=false`, `sello.otorgado=false` con motivo `vencido` y aviso al dueño (`negocio_revisado` con `sello_vencido`). Sin otorgamiento automático: decide el administrador, como hoy.
- **Insignia (nv-76):** el JSON agrega `vigente_hasta`; un Sello vencido = 404.
- **Criterios públicos:** `GET /api/v1/sello` (sin sesión) → `{criterios[{nombre, descripcion, meta}], vigencia_meses}` para `34_Sello`; caché pública 1 día.
- **Ficha del lugar:** `sello_excelencia` sigue igual; no se publica `vigente_hasta` al público.

## Contratos
| Método | Ruta | Auth | Respuestas |
|---|---|---|---|
| GET | `/api/v1/sello` | no | 200 |
| GET | `/api/v1/negocios/{id}/panel` | JWT dueño | cambia `avance_sello` |
| GET | `/api/v1/negocios/{id}/resenas/resumen` | JWT dueño | agrega `tasa_respuesta` |
| PUT | `/api/v1/admin/negocios/{id}/sello` | JWT administrador | agrega `vigente_hasta` |
| GET | `/api/v1/negocios/{id}/sello/insignia` | JWT dueño | agrega `vigente_hasta` |

## Tests
- Unit: cada criterio en su borde (4.49/4.5, 49/50, 89 %/90 %, menos de 10 reseñas en 90 días); cálculo de `vigente_hasta`.
- Integration: avance con reseñas y respuestas sembradas; vencimiento nocturno con y sin renovación; aviso al dueño; insignia con `vigente_hasta` y 404 al vencer.
- Security: `GET /sello` sin datos de negocios; `vigente_hasta` solo para el dueño y el administrador.
- Contract: `GET /sello`, panel y resumen con los campos nuevos.

## Criterios
- Los criterios que ve el dueño son los mismos que muestra `34_Sello`.
- Ningún Sello sigue vigente después de su fecha sin una renovación del administrador.

## Límites
- Sin Sello automático ni niveles de Sello; la decisión sigue siendo del administrador.

## Aprobación y Control de Cambios
- **Estado:** 🚧 Borrador (2026-11-21), sin ejecutar.
- **Revisores:** Osmar Reséndiz (pendiente de aprobar)
- **Fecha:** —
- **Decisiones para el revisor:** (1) cambiar la regla de nv-56 por la de la maqueta (o, al revés, ajustar la maqueta); (2) ventana de 12 meses para el promedio y las 50 reseñas; (3) vigencia de 6 meses y renovación manual; (4) se conservan los criterios de 60 días aprobado y cero rechazos de moderación.
- **Detalle:** `specs/nv-80/checklists/gates.md` al ejecutar.
