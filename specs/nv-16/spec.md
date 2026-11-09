# NV-16 — Versión de la API y versión mínima de la app

## Contexto
La app debe saber si su versión sigue vigente y la API debe poder retirar rutas sin romper a nadie. **Origen**: TEC-29, TEC-30, US-001, US-003. **Depende de**: NV-05, NV-11.

## Requisitos
1. **Versionado:** todas las rutas viven bajo `/api/v1`.
2. **Configuración pública:** `GET /api/v1/configuracion` devuelve `version_minima_app`, `version_actual_app`, `version_terminos`, `version_aviso_privacidad` y `mantenimiento`.
3. **Versión mínima:** la app manda `X-App-Version` (semver).

## Diseño
- **Configuración:** valores por entorno (`APP_VERSION_MINIMA`, `APP_VERSION_ACTUAL`, `TERMINOS_VERSION`, `AVISO_VERSION`, `MODO_MANTENIMIENTO`).

## Contratos
| Método | Ruta | Auth | Respuestas |
|---|---|---|---|
| GET | `/api/v1/configuracion` | no | 200, 304 |

## Tests
- `Unit/Versión 6`: semver menor, igual y mayor; cabecera ausente; ruta sin versión; rutas exentas del 426.
- `Contract/Configuración 2`: forma de la respuesta y modo de mantenimiento.

## Criterios
- La app puede forzar la actualización sin romper la autenticación.

## Límites
- La política de `v2` solo se ejercita si hay un cambio incompatible; hoy se prueba con una ruta de ejemplo.

## Aprobación y Control de Cambios
- **Estado:** 🕓 Pospuesta: fuera de la ruta prioritaria hasta la evaluación del 2 de diciembre de 2026 (ver `Memoria.md`)
- **Revisores:** Osmar Reséndiz (pendiente de aprobar)
- **Fecha:** —
- **Decisiones para el revisor antes de ejecutar:** Confirmar el alcance y los valores propuestos en Diseño.
- **Detalle:** la evidencia irá en `specs/nv-16/checklists/gates.md` + `Memoria.md`.
