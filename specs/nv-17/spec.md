# NV-17 — Paginación por cursor y validadores HTTP

## Contexto
Todas las listas nuevas necesitan una paginación común que no se pueda manipular, y los GET públicos deben poder responder 304. **Origen**: TEC-29, TEC-30, US-001, US-003. **Depende de**: NV-16.

## Requisitos
1. **Paginación por cursor** en todas las listas: `?limite=` (20 por defecto, 50 máximo) y `?cursor=` opaco.
2. **Validadores HTTP:** `ETag` e `If-None-Match` en los GET públicos de catálogo (304 si no cambió).

## Diseño
- **Paquetes:** `internal/shared/paginacion` (codifica y firma el cursor con HMAC para que no se pueda manipular), `internal/shared/cache`, `internal/platform/versionado` (middleware de cabeceras y 426), `internal/configuracion`, `internal/consentimientos`.
- **Cursor:** guarda la llave de orden y el `_id` del último elemento, base64url y firmado.

## Contratos
Sin endpoints nuevos.

## Tests
- `Unit/Cursor 8`: ida y vuelta, cursor alterado, cursor de otra ruta, límite 0, 51 y negativo, fin de lista.
- `Integration/ETag 4`: 304 con `If-None-Match` vigente, 200 tras cambiar el recurso, `no-store` en `/usuarios/yo`.

## Criterios
- Toda lista nueva usa el mismo cursor.

## Límites
- Sin límites propios más allá de los ya anotados.

## Aprobación y Control de Cambios
- **Estado:** 🕓 Pospuesta: fuera de la ruta prioritaria hasta la evaluación del 2 de diciembre de 2026 (ver `Memoria.md`)
- **Revisores:** Osmar Reséndiz (pendiente de aprobar)
- **Fecha:** —
- **Decisiones para el revisor antes de ejecutar:** Confirmar el alcance y los valores propuestos en Diseño.
- **Detalle:** la evidencia irá en `specs/nv-17/checklists/gates.md` + `Memoria.md`.
