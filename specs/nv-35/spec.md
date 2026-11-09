# NV-35 — Buscar personas

## Contexto
Buscar por nombre de usuario o teléfono sin revelar quién existe ni exponer datos. **Origen**: US-232, US-233, US-236 y hoja J2 del Excel. **Depende de**: NV-34.

## Requisitos
1. `GET /personas/buscar?q=` busca por nombre de usuario (prefijo) o por teléfono exacto.
2. Límites por usuario: 30 búsquedas y 20 solicitudes por hora.

## Diseño
- **Búsqueda por teléfono:** se normaliza a E.164 y se busca el HMAC con la misma llave de `nv-05`.
- **Anti-enumeración:** respuestas del mismo tamaño y forma exista o no la persona; tiempo no dependiente del resultado.

## Contratos
| Método | Ruta | Auth | Respuestas |
|---|---|---|---|
| GET | `/api/v1/personas/buscar?q=` | JWT | 200, 422, 429 |

## Tests
- `Integration/Búsqueda 10`: por usuario, por teléfono buscable, teléfono no buscable, usuario inexistente (misma respuesta), bloqueado oculto, prefijo, límite por hora.
- `Security/Amigos 6`: ningún teléfono ni correo en respuestas; no se enumeran teléfonos por tiempo o tamaño; no se acepta una solicitud ajena.

## Criterios
- Cumple los requisitos de arriba.

## Límites
- Sin sugerencias ni contactos del teléfono en esta spec.

## Aprobación y Control de Cambios
- **Estado:** 🚧 Borrador revisado (2026-11-08); ruta prioritaria, ola 3: amistades, ML y grafo
- **Revisores:** Osmar Reséndiz (pendiente de aprobar)
- **Fecha:** —
- **Decisiones para el revisor antes de ejecutar:** Confirmar el alcance y los valores propuestos en Diseño.
- **Detalle:** la evidencia irá en `specs/nv-35/checklists/gates.md` + `Memoria.md`.
