# NV-35 — Buscar personas

## Contexto
Buscar por nombre de usuario o teléfono sin revelar quién existe ni exponer datos. **Origen**: US-232, US-233, US-236 y hoja J2 del Excel. **Depende de**: NV-34.

## Requisitos
1. `GET /personas/buscar?q=` busca por nombre de usuario (prefijo) o por teléfono exacto.
2. Límite por usuario: 30 búsquedas por hora (`rate:personas_buscar:{usuario}`). El de 20 solicitudes por hora pasa a nv-36, que crea `POST /amistades`.

## Diseño
- **Módulo:** `internal/personas` (service, handler, DTO); consultas en `internal/usuarios/search.go`. Límite con `Limiter.WindowUser` (ventana fija por usuario del token).
- **Interpretación de `q`:** teléfono si empieza con `+` o trae 10 o más dígitos (espacios, guiones, puntos y paréntesis se quitan; 10 dígitos = +52); si no, prefijo de nombre de usuario en minúsculas (`@` opcional, 3 a 20 de `[a-z0-9_]`). Lo demás es 422.
- **Quién aparece:** cuentas `activo`, nunca quien busca. Por prefijo: `privacidad.buscable_por_usuario: {$ne: false}`, regex anclada sobre `ux_nombre_usuario`, orden por nombre de usuario, hasta 10. Por teléfono: `privacidad.buscable_por_telefono: true`, una sola consulta por `ux_telefono_hash`.
- **Respuesta:** `{resultados[{id, nombre, nombre_usuario, foto_url}]}`, lista vacía si no hay nada; `Cache-Control: no-store`.
- **Búsqueda por teléfono:** se normaliza a E.164 y se busca el HMAC con la misma llave de `nv-05`.
- **Anti-enumeración:** respuestas del mismo tamaño y forma exista o no la persona; tiempo no dependiente del resultado.

## Contratos
| Método | Ruta | Auth | Respuestas |
|---|---|---|---|
| GET | `/api/v1/personas/buscar?q=` | JWT, cuenta activa | 200, 401, 403, 422, 429 |

## Tests
- `Unit` (`internal/personas/service_test.go`): interpretación y normalización de `q`, 422, HMAC del teléfono y exclusión de quien busca.
- `Integration` (`tests/integration/personas_test.go`, 7): por usuario y prefijo, inexistente (misma respuesta), ocultos por privacidad y por estado (bloqueada, eliminación pendiente), por teléfono buscable y no buscable, 422, 401, límite por hora.
- `Security` (`tests/security/personas_security_test.go`, 2): ningún correo ni teléfono en respuestas, teléfono oculto y no registrado con los mismos bytes; 401 y 403 con la cuenta en eliminación pendiente.
- Pasan a nv-36: ocultar a quien bloqueó (relación `bloqueada`) y "no se acepta una solicitud ajena".

## Criterios
- Cumple los requisitos de arriba.

## Límites
- Sin sugerencias ni contactos del teléfono en esta spec.
- Sin ocultar por bloqueo entre personas: la relación `bloqueada` nace en nv-36, que debe filtrar esta búsqueda.
- El tiempo de respuesta no es constante: el teléfono siempre hace una consulta por índice, exista o no, y el límite por hora frena la enumeración.

## Aprobación y Control de Cambios
- **Estado:** 🚧 Borrador con evidencia (2026-11-11); implementada en la rama `nv-35`, ola 3: amistades, ML y grafo
- **Revisores:** Osmar Reséndiz (pendiente de aprobar)
- **Fecha:** —
- **Decisiones para el revisor antes de ejecutar:** Confirmar el alcance y los valores propuestos en Diseño.
- **Detalle:** la evidencia irá en `specs/nv-35/checklists/gates.md` + `Memoria.md`.
