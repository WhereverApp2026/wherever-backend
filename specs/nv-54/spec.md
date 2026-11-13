# NV-54 — Favoritos de negocios y planes

## Contexto
Guardar con el corazón los negocios y planes que le gustan a la persona, verlos en las pestañas Negocios y Planes de P-35 con la etiqueta «Cupón hoy» y si están abiertos, y pintar los corazones llenos en listas y mapa. **Origen**: Roles B5, pantallas P-20 y P-35, hoja «A3 Favoritos» del Excel (responsable de la hoja: Alexander Dominguez Zarate). **Depende de**: NV-05 (usuarios, `RequireActive`, ARCO), NV-17 (cursor firmado), NV-20 y NV-21 (negocios aprobados, `cupones_activos`, `negocios.OpenAt`), NV-29 y NV-30 (planes con `dueno_id`).

## Requisitos
1. `PUT /favoritos/{tipo}/{objetivo_id}` marca un negocio o plan; es idempotente: repetirlo responde igual sin duplicar.
2. `GET /favoritos?tipo=` lista una pestaña, del más reciente al más viejo, paginada; en negocios agrega `cupon_hoy` y `abierto_ahora`, y `con_cupon=true` deja solo los que tienen cupón.
3. `GET /favoritos/ids?tipo=` devuelve solo los ids para pintar corazones.
4. `GET /favoritos/buscar?q=&tipo=` busca por prefijo del nombre solo entre mis favoritos.
5. `DELETE /favoritos/{tipo}/{objetivo_id}` quita el favorito.

## Diseño
- **Paquete `internal/favoritos`:** repositorio, service, handler, DTO y `Privacy` (Purger y Exporter de nv-05). Lee negocios y planes por interfaz; ninguno de ellos importa `favoritos`.
- **Colección `favoritos`** (Excel A3 §1): `usuario_id`, `tipo` (`negocio` o `plan`), `objetivo_id`, `resumen` (`nombre`, `foto_url`, `categoria`; en un plan solo `nombre`), `creado_en`. Índices `ux_favorito` (usuario, tipo, objetivo; único), `ix_lista` (usuario, tipo, `creado_en` descendente) e `ix_objetivo` (objetivo). Se declaran en `scripts/indexes` y en `/ready`.
- **Objetivo válido** (drift: el Excel solo dice «que exista»): un negocio con `estado=aprobado` sin `eliminado_en`, o un plan cuyo `dueno_id` es la persona (los planes aún no se comparten). Un plan ajeno, un negocio en borrador, pausado o borrado y un id inexistente dan el mismo 404.
- **Marcar:** `upsert` con `$setOnInsert` sobre `ux_favorito`; responde 200 con el favorito la primera vez y las siguientes (Excel: PUT idempotente, sin 201). Tope de 500 favoritos por tipo: el 501.º da 409 `limite_favoritos` (el Excel filtra la búsqueda en memoria porque «son pocos»).
- **Lista:** cursor firmado de nv-17 (`Signer.Scope("favoritos", usuario, tipo)`), `limite` 20 por omisión y 50 como máximo. En negocios, una consulta `$in` a `negocios` con los ids de la página da `cupon_hoy` (`cupones_activos > 0`), `abierto_ahora` (`negocios.OpenAt`, hora de Ciudad de México) y refresca `nombre`, `foto_url` y `categoria` del `resumen` (sin reescribir el documento). Un negocio pausado o no aprobado se omite sin borrar el favorito; un objetivo que ya no existe (plan borrado o negocio con `eliminado_en`) se omite y su favorito se borra ahí mismo. Con `con_cupon=true` la página puede traer menos de `limite`; `siguiente` sigue siendo correcto.
- **Ids:** hasta 500, sin paginar y sin caché en Redis (cuota de Upstash).
- **Buscar:** `q` de 2 a 60 caracteres; lee hasta 500 favoritos con `ix_lista` y filtra en memoria por prefijo de `resumen.nombre` sin acentos ni mayúsculas (la normalización de `personas`). Sin `tipo` busca en los dos.
- **Quitar:** borra por `ux_favorito`; quitar uno que no existe también es 204.
- **Sin `If-Match`:** marcar y quitar son idempotentes, no ediciones de campos.
- **Límites de peticiones:** sin política nueva; aplican las globales de `docs/rate-limit-matrix.md`, sin comandos de Redis adicionales.
- **ARCO:** el `Purger` borra los favoritos al eliminar la cuenta; «borrar historial» no los toca. La exportación incluye `favoritos` (`tipo`, `objetivo_id`, `nombre`, `creado_en`).
- **Variables:** ninguna nueva (nada que agregar a Parameter Store).

## Contratos
| Método | Ruta | Auth | Respuestas |
|---|---|---|---|
| PUT | `/api/v1/favoritos/{tipo}/{objetivo_id}` | JWT consumidor | 200, 401, 403, 404, 409, 422 |
| GET | `/api/v1/favoritos?tipo=&con_cupon=&cursor=&limite=` | JWT consumidor | 200, 401, 403, 422 |
| GET | `/api/v1/favoritos/ids?tipo=` | JWT consumidor | 200, 401, 403, 422 |
| GET | `/api/v1/favoritos/buscar?q=&tipo=` | JWT consumidor | 200, 401, 403, 422 |
| DELETE | `/api/v1/favoritos/{tipo}/{objetivo_id}` | JWT consumidor | 204, 401, 403, 422 |

403 = otro rol o cuenta en eliminación pendiente (`usuarios.RequireActive`). 422 = `tipo` distinto de `negocio`/`plan`, id inválido, `q` corto o cursor alterado. Filas de la hoja A3 §2; al ejecutar se copian a `docs/endpoints.md`.

## Tests
- `tests/integration/favoritos_test.go` con MongoDB real: marcar dos veces (un documento), lista por pestaña con orden y cursor, `cupon_hoy`, `abierto_ahora` y `con_cupon`, ids, buscar sin acentos, quitar, negocio pausado (oculto) y plan borrado (se limpia), plan ajeno (404), tope 500 (409), borrar cuenta y exportar.
- `tests/security/favoritos_security_test.go`: las cinco rutas con 401 sin token y 403 con comerciante y con cuenta en eliminación pendiente.
- `tests/database/indexes_test.go`: índices de `favoritos`. `tests/contract`: forma de la lista y `favoritos: []` en la exportación.
- Unit `internal/favoritos/favoritos_test.go`: DTOs, `tipo`, `q`, `limite` y prefijo normalizado.

## Criterios
- El corazón queda igual tras marcar dos veces, y la lista nunca muestra un negocio no aprobado ni un plan ajeno.

## Límites
- Sin contador público de cuántas personas guardaron un negocio (estadísticas A5; `ix_objetivo` queda listo).
- Sin señal de favoritos en el recomendador (nv-23); va en su propia spec.
- Sin favoritos de planes compartidos hasta que existan grupos.
- Sin sincronización sin conexión (nv-47 la excluye).

## Aprobación y Control de Cambios
- **Estado:** 🚧 Borrador (2026-11-12), ola por decidir
- **Revisores:** Osmar Reséndiz (pendiente de aprobar)
- **Fecha:** —
- **Decisiones para el revisor:** solo rol `consumidor` (Excel); plan solo propio; tope 500 por tipo con 409; PUT responde 200 siempre; negocio pausado se oculta sin borrar y objetivo inexistente se limpia al listar.
- **Detalle:** evidencia en `specs/nv-54/checklists/gates.md` + `Memoria.md` al ejecutar.
