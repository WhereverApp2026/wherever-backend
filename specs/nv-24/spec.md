# NV-24 — Recomendaciones y «No me interesa»

## Contexto
La persona recibe «Para ti ahora» y puede descartar lugares para no volver a verlos. **Origen**: US-166, US-025, US-037 y hoja A2 del Excel. **Depende de**: NV-23.

## Requisitos
1. `GET /recomendaciones` devuelve «Para ti ahora»; acepta compañía (solo, pareja, amigos, familia) y hora.
2. `POST` y `DELETE /recomendaciones/{negocio_id}/no-me-interesa`.
3. Si el usuario apagó `aprendizaje_personalizado` (ya existe en privacidad, nv-05) o no dio consentimiento de datos sensibles, el motor no usa ánimo ni alergias guardadas más allá del filtro duro de alergias que él mismo indicó.

## Diseño
- **Paquete:** `internal/recomendaciones` (repositorio, service, handler, DTO y ARCO) sobre el motor de nv-23 (`recomendador.Recomendar`, `negocios.Repository.Candidates`, `ParseAlergias`, `DietasDe`, `PesosPorDefecto`). La tarjeta reutiliza la de `GET /lugares` (`negocios.ItemOf`) más `puntaje`, `razones[]` y `por_que`.
- **«No me interesa»:** colección `descartes_usuario` `{_id, usuario_id, negocio_id, creado_en}` con índice único `ux_usuario_negocio` (usuario, negocio); upsert idempotente. Se excluyen los 1000 más recientes con `$nin` dentro de `$geoNear.query` (filtro duro en la consulta). Entra en ARCO: se exporta y se borra con «Borrar mi historial» y con la cuenta.
- **Petición:** `lat` y `lng` obligatorios (el motor necesita un punto), `radio_m` (5 km por defecto, 20 km máx.), `compania` y `hora` (HH:MM de hoy en `America/Mexico_City`; sin ella, ahora), `limite` 10 por defecto y 50 máx.
- **Compañía:** suma términos a los gustos (`solo`/`individual`, `pareja`/`romantico`, `amigos`/`grupos`, `familia`/`familiar`) que coinciden con giro o etiquetas; no es filtro duro.
- **Privacidad (requisito 3):** alergias solo con `datos_sensibles` (regla de nv-05: sin consentimiento no se usan) y solo como filtro duro, nunca en puntaje ni en `por_que`; dietas del catálogo, siempre filtro duro; gustos y presupuesto guardados, solo con `aprendizaje_personalizado`. El ánimo aún no es entrada del motor.
- **Pesos:** `cmd/api` valida `pesos_recomendador.json` al arrancar (`recomendaciones.NewService`); con pesos inválidos la API no levanta.

## Contratos
| Método | Ruta | Auth | Respuestas |
|---|---|---|---|
| GET | `/api/v1/recomendaciones` | JWT | 200, 401, 403, 422, 503 |
| POST | `/api/v1/recomendaciones/{negocio_id}/no-me-interesa` | JWT | 204, 401, 403, 404 |
| DELETE | `/api/v1/recomendaciones/{negocio_id}/no-me-interesa` | JWT | 204, 401, 403, 404 (no estaba descartado o es de otro usuario) |

Detalle vivo en `docs/endpoints.md`.

## Tests
- Unit `internal/recomendaciones/contexto_test.go`: privacidad (aprendizaje, consentimiento, alergia fuera del catálogo), compañía, hora, validación de la consulta, tarjeta y pesos inválidos.
- `tests/integration/recomendaciones_test.go` (7) con MongoDB real: ranking esperado con los 10 negocios de nv-23 y su porqué, «No me interesa» y su deshacer, negocio inexistente 404, aprendizaje apagado, usuario sin gustos, 422/401 y ARCO.
- `tests/security/recomendaciones_security_test.go` (3): 401 sin sesión, un usuario no ve ni cambia los descartes de otro, cuenta en eliminación 403.
- `tests/database/indexes_test.go`: `ux_usuario_negocio` único.

## Criterios
- Cada tarjeta trae su porqué.

## Límites
- Sin límites propios más allá de los ya anotados.

## Aprobación y Control de Cambios
- **Estado:** 🚧 Borrador con evidencia (2026-11-11): implementada en la rama `nv-24` sobre `nv-23`; falta la firma y el CI del PR
- **Revisores:** Osmar Reséndiz (pendiente de aprobar)
- **Fecha:** —
- **Decisiones para el revisor:** confirmar la regla de privacidad, el efecto de la compañía y el 404 del DELETE sin descarte (Diseño).
- **Detalle:** la evidencia irá en `specs/nv-24/checklists/gates.md` + `Memoria.md`.
