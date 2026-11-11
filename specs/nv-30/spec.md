# NV-30 — Guardar, leer, editar y repetir planes

## Contexto
La persona guarda sus planes, los edita, los borra y repite los que le gustaron. **Origen**: US-055, US-058 y hoja A4 del Excel. **Depende de**: NV-29.

## Requisitos
1. `POST /planes` guarda un plan (generado o hecho a mano) con nombre, fecha, personas, presupuesto y movilidad.
2. Editar: cambiar nombre, fecha, personas, presupuesto o movilidad; agregar, mover, cambiar duración, reordenar y quitar paradas.
3. `GET /planes?pestana=` lista por pestaña (`proximos`, `guardados`, `pasados`; grupales llega con nv-37), paginada por cursor.
4. **Repetir (US-058):** `POST /planes/{id}/clonar` duplica el plan propio con una fecha nueva; guarda el nombre editable.
5. Borrado lógico del plan; un plan con integrantes de grupo solo lo borra el dueño.

## Diseño
- **Modelo `planes`:** `dueno_id`, `nombre`, `fecha`, `personas`, `presupuesto`, `movilidad`, `paradas[{id, negocio_id, llega, minutos, precio, nota}]`, `traslados[{desde, hasta, minutos, km}]`, `costo_total`, `costo_por_persona`, `estado` (`activo`, `borrado`), `origen` (`manual`, `ia`, `reglas`),…
- **Privacidad:** un plan solo lo lee su dueño; el acceso de integrantes llega con nv-37. El modelo, `Recalcular` y el repositorio son de nv-29 (`internal/planes`); esta spec agrega DTO, service y handler en el mismo paquete.

## Contratos
Todas con JWT (sin token = 401). Un plan ajeno, borrado o con id mal formado responde 404 (no 403: no se revela que existe). Las ediciones de un plan existente piden `If-Match` con la versión: falta o mal formada = 400, versión vieja = 409 (como el perfil de nv-05). Con la cuenta en eliminación pendiente las escrituras dan 403.

| Método | Ruta | Respuestas |
|---|---|---|
| POST | `/api/v1/planes` | 201, 400, 401, 422 |
| GET | `/api/v1/planes?pestana=&cursor=&limite=` | 200, 401, 422 |
| GET | `/api/v1/planes/{id}` | 200, 401, 404 |
| PATCH | `/api/v1/planes/{id}` | 200, 400, 401, 404, 409, 422 |
| DELETE | `/api/v1/planes/{id}` | 204, 400, 401, 404, 409 |
| POST | `/api/v1/planes/{id}/paradas` | 201, 400, 401, 404, 409, 422 |
| PATCH | `/api/v1/planes/{id}/paradas/{parada_id}` | 200, 400, 401, 404, 409, 422 |
| PUT | `/api/v1/planes/{id}/paradas/orden` | 200, 400, 401, 404, 409, 422 |
| DELETE | `/api/v1/planes/{id}/paradas/{parada_id}` | 200, 400, 401, 404, 409, 422 |
| POST | `/api/v1/planes/{id}/clonar` | 201, 401, 404, 422 |

- `fecha` es un día (`2006-01-02`) en `America/Mexico_City`; `llega` es RFC 3339. Cambiar la fecha (PATCH o clonar) mueve todas las paradas los mismos días.
- Las paradas siempre van en orden de llegada (nv-29). «Reordenar» reparte las mismas horas de llegada en el orden pedido; si una parada ya no cabe o el negocio está cerrado, 422.
- Pestañas: `proximos` (fecha desde hoy, ascendente, por defecto), `pasados` (antes de hoy, descendente) y `guardados` (todos los activos, descendente). Un cursor de otra pestaña da 422.
- Toda edición pasa por `Calculadora.Aplicar`: negocio no aprobado, cerrado o paradas encimadas = 422 con la parada en `campos`.

## Tests
- `Unit/Planes`: validación de los DTO (7 paradas, movilidad, fechas), aplicar el PATCH, mover fechas y reordenar.
- `Integration/PlanesAPI`: crear, leer, editar cada campo, agregar, editar, reordenar y quitar paradas, clonar con fecha nueva, borrar, pestañas con cursor; sin `If-Match` = 400 y versión vieja = 409.
- `Security/Planes`: sin token = 401 en cada ruta; leer, editar, clonar o borrar el plan de otro = 404 y no lo cambia; negocio no aprobado no entra; 7 paradas = 422; la respuesta no lleva `dueno_id`.

## Criterios
- Repetir un plan no copia reservas ni integrantes (aún no existen; el clon solo lleva paradas con ids nuevos).
- El dueño es el único con acceso; toda consulta filtra por dueño y estado.

## Límites
- Sin alternativas por parada ni compartir en esta spec. Planes grupales e integrantes llegan con nv-37.

## Aprobación y Control de Cambios
- **Estado:** 🚧 Borrador con evidencia (2026-11-11): implementada en la rama `nv-30` sobre `nv-29`; falta el CI del PR y la firma.
- **Revisores:** Osmar Reséndiz (pendiente de aprobar)
- **Fecha:** —
- **Decisiones para el revisor:** 409/400 en lugar de 412/428 (como nv-05); plan ajeno = 404 (no 403); reordenar reparte las horas de llegada; `guardados` = todos los activos hasta que exista una marca de guardado.
- **Detalle:** evidencia en `specs/nv-30/checklists/gates.md` + `Memoria.md`.
