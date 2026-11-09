# NV-22 — Búsqueda y autocompletar

## Contexto
Quien ya sabe a dónde ir debe encontrar el negocio escribiendo pocas letras, aunque se equivoque. **Origen**: US-152, US-027, US-026 y hoja A2 del Excel. **Depende de**: NV-21.

## Requisitos
1. **Autocompletar** desde 2 letras, tolerante a acentos y mayúsculas, y **búsqueda** `GET /lugares/buscar?q=` con índice de texto en español y tolerancia a errores menores.
2. Búsquedas recientes del usuario (`GET/DELETE /busquedas/recientes`), solo con sesión.

## Diseño
- Sin decisiones de diseño propias: sigue lo definido arriba.

## Contratos
| Método | Ruta | Auth | Respuestas |
|---|---|---|---|
| GET | `/api/v1/lugares/autocompletar?q=` | no | 200, 422 |
| GET | `/api/v1/lugares/buscar?q=` | no | 200, 422 |
| GET / DELETE | `/api/v1/busquedas/recientes` | JWT | 200, 204 |

## Tests
- `Integration/Búsqueda 8`: acentos, mayúsculas, prefijo de 2 letras, error de una letra, sin resultados, texto inyectado.

## Criterios
- La búsqueda encuentra con acentos y errores menores.

## Límites
- Sin límites propios más allá de los ya anotados.

## Aprobación y Control de Cambios
- **Estado:** 🕓 Pospuesta: fuera de la ruta prioritaria hasta la evaluación del 2 de diciembre de 2026 (ver `Memoria.md`)
- **Revisores:** Osmar Reséndiz (pendiente de aprobar)
- **Fecha:** —
- **Decisiones para el revisor antes de ejecutar:** Confirmar el alcance y los valores propuestos en Diseño.
- **Detalle:** la evidencia irá en `specs/nv-22/checklists/gates.md` + `Memoria.md`.
