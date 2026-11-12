# NV-22 — Búsqueda y autocompletar

## Contexto
Quien ya sabe a dónde ir debe encontrar el negocio escribiendo pocas letras, aunque se equivoque. **Origen**: US-152, US-027, US-026 y hoja A2 del Excel. **Depende de**: NV-21.

## Requisitos
1. **Autocompletar** desde 2 letras, tolerante a acentos y mayúsculas, y **búsqueda** `GET /lugares/buscar?q=` con índice de texto en español y tolerancia a errores menores.
2. Búsquedas recientes del usuario (`GET/DELETE /busquedas/recientes`), solo con sesión.

## Diseño
- **Paquete** `internal/busquedas` (repositorio, service, handler, DTO y ARCO). Lee `negocios` siempre con `negocios.VisibleFilter()` en la consulta; no crea índices en `negocios` (usa `ix_texto` e `ix_autocompletar` de nv-20).
- **Autocompletar:** `nombre_normalizado` con una palabra que empieza con `q` normalizado (`(^| )` + texto escapado con `regexp.QuoteMeta`); primero los que empiezan así, luego por calificación. `limite` 8 por defecto, máx. 20.
- **Búsqueda:** `q` se reduce a palabras de letras y dígitos (`Palabras`): ni comillas ni guiones de `$text` ni operadores llegan a MongoDB. Primero `$text` en español (raíces: «taco» encuentra «tacos»); si no llena la página, se completan con errores menores contra nombre, etiquetas y giro de hasta 1000 aprobados en Go (Damerau: 1 letra de 4 a 7, 2 de 8 o más, prefijos con error; todas las palabras deben coincidir) y la respuesta lleva `aproximada: true`. Corrección del hallazgo: el índice de texto de MongoDB no tolera errores; Atlas Search no corre en Testcontainers, así que se compara en Go como el plan B de nv-51. Con mucho más catálogo, cambiar a trigramas o Atlas Search.
- **Recientes:** colección `busquedas_recientes` `{usuario_id, texto, texto_normalizado, buscado_en}`; se guarda al buscar con sesión (no al autocompletar); repetir solo sube la fecha (`ux_usuario_texto`); quedan las 10 más nuevas y cada una vive 90 días (`ttl_buscado_en`). Entra en ARCO (exportar, borrar historial y cuenta). Si guardarla falla, la búsqueda responde igual.
- **Privacidad:** respuestas `no-store`; el error de MongoDB no se devuelve ni se registra (podría repetir lo escrito); el log de un fallo al guardar no lleva el texto.
- **Redis:** sin comandos nuevos fuera del limitador global.

## Contratos
| Método | Ruta | Auth | Respuestas |
|---|---|---|---|
| GET | `/api/v1/lugares/autocompletar?q=` | no | 200, 422 |
| GET | `/api/v1/lugares/buscar?q=` | opcional | 200, 401, 422, 503 |
| GET / DELETE | `/api/v1/busquedas/recientes` | JWT | 200, 204, 401, 403 |

## Tests
- `Integration/Búsqueda 8` (`tests/integration/busqueda_test.go`): autocompletar con 2 letras, acentos y mayúsculas; búsqueda con acentos y raíces; error de una letra y letras cambiadas; sin resultados; texto inyectado y menos de 2 letras (422); recientes con sesión (10, sin duplicar, borrar); recientes sin sesión (401); ARCO.
- `Unit` (`internal/busquedas/fuzzy_test.go`): palabras, tolerancia, distancia y orden de los aproximados.
- `Database`: índices de `busquedas_recientes`. `Contract`: llave `busquedas_recientes` en la exportación.

## Criterios
- La búsqueda encuentra con acentos y errores menores.

## Límites
- Sin límites propios más allá de los ya anotados.

## Aprobación y Control de Cambios
- **Estado:** 🚧 Borrador con evidencia (2026-11-12): implementada en la rama `nv-22`; local en verde (build, vet, Unit, lint, critic, `check_sdd`, `check_endpoints`); Integration, Security, Database y Contract las corre el CI del PR.
- **Revisores:** Osmar Reséndiz (pendiente de aprobar)
- **Fecha:** —
- **Decisiones para el revisor:** confirmar la tolerancia en Go con tope de 1000 negocios, guardar recientes sin pedir `aprendizaje_personalizado` (es historial visible y borrable) y la vida de 90 días.
- **Detalle:** la evidencia irá en `specs/nv-22/checklists/gates.md` + `Memoria.md`.
