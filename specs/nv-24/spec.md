# NV-24 — Recomendaciones y «No me interesa»

## Contexto
La persona recibe «Para ti ahora» y puede descartar lugares para no volver a verlos. **Origen**: US-166, US-025, US-037 y hoja A2 del Excel. **Depende de**: NV-23.

## Requisitos
1. `GET /recomendaciones` devuelve «Para ti ahora»; acepta compañía (solo, pareja, amigos, familia) y hora.
2. `POST` y `DELETE /recomendaciones/{negocio_id}/no-me-interesa`.
3. Si el usuario apagó `aprendizaje_personalizado` (ya existe en privacidad, nv-05) o no dio consentimiento de datos sensibles, el motor no usa ánimo ni alergias guardadas más allá del filtro duro de alergias que él mismo indicó.

## Diseño
- **«No me interesa»:** colección `descartes_usuario` con índice único (usuario, negocio).

## Contratos
| Método | Ruta | Auth | Respuestas |
|---|---|---|---|
| GET | `/api/v1/recomendaciones` | JWT | 200, 422 |
| POST / DELETE | `/api/v1/recomendaciones/{negocio_id}/no-me-interesa` | JWT | 204, 404 |

## Tests
- `Integration/Recomendaciones 8` con MongoDB real: ranking esperado con 10 negocios fijos, «No me interesa» y su deshacer, aprendizaje apagado, usuario sin gustos.
- `Security/Recomendador 3`: un usuario no ve ni cambia los descartes de otro.

## Criterios
- Cada tarjeta trae su porqué.

## Límites
- Sin límites propios más allá de los ya anotados.

## Aprobación y Control de Cambios
- **Estado:** 🚧 Borrador revisado (2026-11-08); ruta prioritaria, ola 1: catálogo y recomendaciones
- **Revisores:** Osmar Reséndiz (pendiente de aprobar)
- **Fecha:** —
- **Decisiones para el revisor antes de ejecutar:** Confirmar el alcance y los valores propuestos en Diseño.
- **Detalle:** la evidencia irá en `specs/nv-24/checklists/gates.md` + `Memoria.md`.
