# NV-19 — Aceptación de términos y aviso de privacidad

## Contexto
La app debe registrar qué versión de términos aceptó cada persona y volver a pedirla si cambia. **Origen**: TEC-29, TEC-30, US-001, US-003. **Depende de**: NV-18.

## Requisitos
1. **Aceptación de términos (US-003):** `POST /api/v1/usuarios/yo/consentimientos` guarda la versión de términos y de aviso aceptadas, la fecha y la casilla opcional de promociones.
2. El aviso incluye la sección de terceros de `docs/aviso-privacidad.md`, con el proveedor externo de IA (Gemini, nv-26); un cambio en esa sección sube la versión del aviso.

## Diseño
- **Consentimientos:** colección `consentimientos` de solo agregar (usuario, versiones, fecha, promociones).

## Contratos
| Método | Ruta | Auth | Respuestas |
|---|---|---|---|
| POST | `/api/v1/usuarios/yo/consentimientos` | JWT | 201, 401, 422 |

## Tests
- `Integration/Consentimientos 5`: aceptar, cambiar la versión y ver `consentimiento_vigente` falso, aceptar de nuevo, historial intacto.

## Criterios
- Cumple los requisitos de arriba.

## Límites
- Sin límites propios más allá de los ya anotados.

## Aprobación y Control de Cambios
- **Estado:** 🕓 Pospuesta: fuera de la ruta prioritaria hasta la evaluación del 2 de diciembre de 2026 (ver `Memoria.md`)
- **Revisores:** Osmar Reséndiz (pendiente de aprobar)
- **Fecha:** —
- **Decisiones para el revisor antes de ejecutar:** Confirmar el alcance y los valores propuestos en Diseño.
- **Detalle:** la evidencia irá en `specs/nv-19/checklists/gates.md` + `Memoria.md`.
