# NV-43 — Cola de revisión y reglas de moderación

## Contexto
Lo dudoso espera a un administrador, que lo aprueba o rechaza y ajusta las reglas. **Origen**: TEC-31 y hoja A7 del Excel. **Depende de**: NV-42.

## Requisitos
1. Lo dudoso entra a **cuarentena** y no se publica hasta que un administrador lo apruebe o rechace con motivo.
2. Endpoints de administrador: cola, detalle con puntajes, aprobar o rechazar, ver y ajustar reglas (umbrales y palabras).
3. Cada decisión queda registrada con quién y cuándo.

## Diseño
- **Colecciones:** `moderacion_cola` (contenido, tipo, estado, puntajes, decisión, revisor) y `moderacion_reglas`.
- **Sin endpoint interno HTTP:** al ser un monolito, `POST /interno/moderacion` del Excel no se expone; se usa el paquete.
- **Permisos:** `rekognition:DetectModerationLabels` y, para S3, lectura del bucket de nv-44, en el boundary y el rol de la instancia.

## Contratos
| Método | Ruta | Auth | Respuestas |
|---|---|---|---|
| GET | `/api/v1/admin/moderacion` | administrador | 200 |
| GET | `/api/v1/admin/moderacion/{id}` | administrador | 200, 404 |
| PATCH | `/api/v1/admin/moderacion/{id}` | administrador | 200, 404, 409 |
| GET / POST / PATCH | `/api/v1/admin/moderacion/reglas` | administrador | 200, 201, 404 |

## Tests
- `Integration/Cola 8`: aprobar, rechazar con motivo, doble decisión (409), consumidor recibe 403.

## Criterios
- Los umbrales se ajustan sin recompilar.

## Límites
- Sin límites propios más allá de los ya anotados.

## Aprobación y Control de Cambios
- **Estado:** 🚧 Borrador (2026-11-08), con hallazgos en `Memoria.md`; ola 6: imágenes y moderación, después de la ruta prioritaria (ver `Memoria.md`)
- **Revisores:** Osmar Reséndiz (pendiente de aprobar)
- **Fecha:** —
- **Decisiones para el revisor antes de ejecutar:** Confirmar el alcance y los valores propuestos en Diseño.
- **Detalle:** la evidencia irá en `specs/nv-43/checklists/gates.md` + `Memoria.md`.
