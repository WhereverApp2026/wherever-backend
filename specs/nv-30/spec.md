# NV-30 — Guardar, leer, editar y repetir planes

## Contexto
La persona guarda sus planes, los edita, los borra y repite los que le gustaron. **Origen**: US-055, US-058 y hoja A4 del Excel. **Depende de**: NV-29.

## Requisitos
1. `POST /planes` guarda un plan (generado o hecho a mano) con nombre, fecha, personas, presupuesto y movilidad.
2. Editar: cambiar nombre, fecha, personas, presupuesto o movilidad; agregar, mover, cambiar duración, reordenar y quitar paradas.
3. `GET /planes?pestana=` lista por pestaña (próximos, guardados, pasados; grupales llega con nv-37), paginada.
4. **Repetir (US-058):** `POST /planes/{id}/clonar` duplica el plan propio con una fecha nueva; guarda el nombre editable.
5. Borrado lógico del plan; un plan con integrantes de grupo solo lo borra el dueño.

## Diseño
- **Modelo `planes`:** `dueno_id`, `nombre`, `fecha`, `personas`, `presupuesto`, `movilidad`, `paradas[{id, negocio_id, llega, minutos, precio, nota}]`, `traslados[{desde, hasta, minutos, km}]`, `costo_total`, `costo_por_persona`, `estado` (`activo`, `borrado`), `origen` (`manual`, `ia`, `reglas`),…
- **Privacidad:** un plan solo lo lee su dueño; el acceso de integrantes llega con nv-37.

## Contratos
| Método | Ruta | Auth | Respuestas |
|---|---|---|---|
| POST | `/api/v1/planes` | JWT | 201, 422 |
| GET | `/api/v1/planes?pestana=` | JWT | 200 |
| GET | `/api/v1/planes/{id}` | JWT dueño | 200, 403, 404 |
| PATCH | `/api/v1/planes/{id}` | JWT dueño | 200, 404, 412, 422, 428 |
| DELETE | `/api/v1/planes/{id}` | JWT dueño | 204, 404 |
| POST | `/api/v1/planes/{id}/paradas` | JWT dueño | 201, 422 |
| PATCH | `/api/v1/planes/{id}/paradas/{parada_id}` | JWT dueño | 200, 422 |
| PUT | `/api/v1/planes/{id}/paradas/orden` | JWT dueño | 200, 422 |
| DELETE | `/api/v1/planes/{id}/paradas/{parada_id}` | JWT dueño | 200, 404 |
| POST | `/api/v1/planes/{id}/clonar` | JWT dueño | 201, 404 |

## Tests
- `Integration/Planes 14` con MongoDB real: crear, editar cada campo, reordenar, quitar, clonar con fecha nueva, pestañas, cursor.
- `Integration/Concurrencia 4`: dos ediciones simultáneas, sin `If-Match` → 428, versión vieja → 412.
- `Security/Planes 5`: leer, editar o borrar el plan de otro da 404; negocio no aprobado no entra; entrada con 7 paradas se rechaza.

## Criterios
- Repetir un plan no copia reservas ni integrantes.
- El dueño es el único con acceso.

## Límites
- Sin alternativas por parada ni compartir en esta spec.

## Aprobación y Control de Cambios
- **Estado:** 🚧 Borrador revisado (2026-11-08); ruta prioritaria, ola 2: planes con IA (hallazgo C1)
- **Revisores:** Osmar Reséndiz (pendiente de aprobar)
- **Fecha:** —
- **Decisiones para el revisor antes de ejecutar:** Confirmar el alcance y los valores propuestos en Diseño.
- **Detalle:** la evidencia irá en `specs/nv-30/checklists/gates.md` + `Memoria.md`.
