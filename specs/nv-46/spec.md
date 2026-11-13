# NV-46 — Descarga de cambios

## Contexto
La app baja solo lo que cambió desde su última sincronización, incluidos los borrados. **Origen**: TEC-33 y hoja A6 del Excel. **Depende de**: NV-29/30 (planes) y NV-05 (perfil).

## Requisitos
1. `GET /sync/cambios?desde=` devuelve, paginado, lo que cambió para esta persona desde un punto (planes y ajustes), incluidos los borrados.
2. Un reloj atrasado en el teléfono no rompe la sincronización.
3. Solo se sincronizan datos de la propia persona.

## Diseño
- **Paquete** `internal/sincronizacion`: repositorio (contador y borrados), service, handler, DTO y ARCO.
- **Contador por usuario** `sync_estado{_id: usuario, seq}`: `Next` sube con un `findOneAndUpdate` (`$inc`, upsert), así dos escrituras simultáneas nunca reciben el mismo número ni dejan huecos. `planes.Service.WithSync` numera cada alta, edición (incluida la ganadora de una votación, nv-39) y borrado en `planes.sync_seq` (`ix_dueno_sync`).
- **Borrados:** colección `borrados{usuario_id, tipo, entidad_id, sync_seq, expira_en}` con TTL de 30 días (`ix_usuario_seq`, `ttl_borrado`). Borrar un plan guarda su borrado con el mismo número.
- **Ajustes:** el perfil ya tiene `version`, que sube en cada edición (contador del servidor, no reloj). `ajustes` (preferencias, privacidad y gustos con la forma de `GET /usuarios/yo`) sale en la descarga completa y cuando la versión cambió.
- **Cursor `desde`:** el `siguiente` de la respuesta anterior, firmado con `CURSOR_HMAC_KEY` (nv-17) y atado al usuario. Guarda el último número entregado, la versión del perfil y la hora **del servidor** en que se dio. Más de 30 días = 410 con `campos[{campo: desde, mensaje: requiere_completa}]`: sus borrados ya vencieron y la app baja todo sin `desde`.
- **Página:** planes y borrados se unen en orden de número; `limite` 100 por defecto (máx. 200); `hay_mas` pide seguir.
- **Corrección de drift (2026-11-12):** la dependencia con la caché de nv-18 no era real. «Ajustes» se sincroniza por la versión del perfil, sin numerarlo con el contador. Solo los planes propios: los de grupos donde la persona es integrante no son suyos.

## Contratos
Fila completa en `docs/endpoints.md`.

| Método | Ruta | Auth | Respuestas |
|---|---|---|---|
| GET | `/api/v1/sync/cambios?desde=&limite=` | JWT | 200, 401, 410, 422 |

## Tests
- Integration `tests/integration/sync_test.go`: descarga completa con ajustes; sin cambios; edición, borrado y alta en orden de cambio; ajustes al cambiar la privacidad; paginación con `hay_mas` sin repetidos; cursor de hace más de 30 días = 410; solo lo propio y el cursor de otra persona = 422.
- Database: `ix_dueno_sync` en `planes` y los índices de `borrados`.

## Criterios
- El reloj del teléfono no influye.

## Límites
- Descarga completa tras 30 días sin conexión.
- Dos ediciones del mismo usuario en el mismo milisegundo pueden guardarse en orden distinto al de su número; quien sincroniza justo en medio puede saltarse la primera hasta su siguiente edición o la siguiente descarga completa. Es una ventana de milisegundos y se acepta.

## Aprobación y Control de Cambios
- **Estado:** 🚧 Borrador revisado (2026-11-12) con la corrección de drift; en ejecución en la rama `nv-46`.
- **Revisores:** Osmar Reséndiz (pendiente de aprobar)
- **Fecha:** —
- **Decisiones para el revisor:** ajustes por versión del perfil; solo planes propios; 410 con `campos`.
- **Detalle:** la evidencia irá en `specs/nv-46/checklists/gates.md` + `Memoria.md`.
