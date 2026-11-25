# NV-93 — Lista de cobros del negocio

## 1. Objetivo
El dueño del negocio necesita ver todos sus cobros en su panel. Hoy solo existe el cobro de una reserva (`GET /negocios/{id}/reservas/{rid}/cobro`), el saldo y las liquidaciones semanales (nv-60). Esta spec agrega la lista paginada de cobros con filtros y totales del rango. **Origen**: revisión de lo que pide la app (2026-11-25). **Depende de**: NV-60 (pagos y liquidaciones), NV-17 (cursor firmado).

## 2. Alcance / No-alcance
- **Dentro:** `GET /api/v1/negocios/{id}/cobros` (repositorio, service, DTO, ruta), índice `ix_pago_negocio`, pruebas y documentos.
- **Fuera:** exportar los cobros a CSV o PDF, cambiar el cálculo del desglose (sigue `pagos.DesgloseDe`) y los datos de quien pagó (ver D-1).

## 3. Contratos API
| Método | Ruta | Auth | Rate | Códigos |
|---|---|---|---|---|
| GET | `/api/v1/negocios/{id}/cobros?cursor=&limite=&desde=&hasta=&estado=` | JWT dueño del negocio | global | 200/401/403/404/422 |

- **Respuesta:** `PageResult` más `totales`: `{items: [{pago_id, reserva_id, fecha_reserva, fecha_cobro, anticipo, reembolsos, comision, neto, estado, tarjeta?: {marca, ultimos4}, liquidacion_semana?}], siguiente_cursor, limite, totales: {pagos, anticipo, reembolsos, comision, neto}}`. Montos en centavos.
- **Orden:** del cobro más nuevo al más viejo (`creado_en` y `_id` para desempatar). El cursor va firmado con el negocio y los filtros: un cursor de otro negocio o de otros filtros = 422.
- **Filtros:** `desde` y `hasta` (`AAAA-MM-DD`, hora de la Ciudad de México, ambos incluidos) sobre la fecha del cobro; `hasta` anterior a `desde` = 422. `estado` es uno de `requiere_pago`, `procesando`, `aprobado`, `rechazado`, `reembolsado_parcial`, `reembolsado`. `limite` de 1 a 50 (20 por defecto).
- **Fechas:** `fecha_reserva` y `fecha_cobro` en RFC 3339 (UTC), como `fecha_hora` de las reservas; `fecha_cobro` es la creación del pago. `liquidacion_semana` es el lunes de la liquidación (`AAAA-MM-DD`) y falta mientras el pago no se liquida.
- **Totales:** suman los pagos cobrados (`aprobado`, `reembolsado_parcial`, `reembolsado`) que cumplen los filtros, en todo el rango y no solo en la página. Los pagos sin cobrar (`requiere_pago`, `procesando`, `rechazado`) salen en la lista pero no en los totales.
- **404:** negocio inexistente, borrado o de otro dueño (igual que `negocioPropio` en nv-60). **403:** token sin rol `comerciante`. **401:** sin token.

## 4. Modelo de datos
- Sin colecciones ni campos nuevos; se lee `pagos`.
- Índice nuevo `ix_pago_negocio` en `pagos`: `{negocio_id: 1, creado_en: -1, _id: -1}`. Cubre el filtro por negocio, el rango de fechas y el orden con su desempate. Va en `pagos.Indexes()` (lo crea `scripts/indexes`) y en `tests/database`.

## 5. Seguridad
- Solo el dueño: `RequireRoles(comerciante)` + `RequireActive` en la ruta y la propiedad del negocio en el service (`negocioPropio`).
- **D-1 — sin datos de quien pagó.** Cada fila trae ids, montos, estado y la marca y últimos 4 de la tarjeta (lo que ya se guarda en `pagos`). No trae `usuario_id`, nombre, correo ni ids de Stripe (`payment_intent_id`, `refund_id`). Porqué: el panel cruza el cobro con su reserva por `reserva_id`, donde el negocio ya ve lo que nv-59 le muestra; repetir el nombre aquí obligaría a leer `usuarios` por cada fila sin ganar nada.
- `Cache-Control: no-store`, como el resto de rutas de pagos.

## 6. Tests / Gates
- **Unit:** `internal/pagos/cobros_test.go`: validación de la consulta (`estado`, fechas, `limite`), rango de fechas en la zona del negocio y totales solo de pagos cobrados.
- **Integration:** `tests/integration/pagos_test.go` `TestPagos_CobrosDelNegocio`: varios pagos (uno de otro negocio), paginación con cursor, filtros por estado y por fechas, totales, cursor ajeno = 422, `hasta < desde` = 422 y negocio de otro dueño = 404.
- **Security:** la ruta entra en `rutasPagos` de `tests/security/pagos_security_test.go` (401 sin token, 403 sin rol de comerciante).
- **Database:** `ix_pago_negocio` en `tests/database/indexes_test.go`.
- **Contract:** `pagos/cobros_200` en `tests/contract/pagos_cobros_contract_test.go`.

## 7. Plan de entrega
- Crear el índice en Atlas antes o justo después de desplegar (`scripts/indexes`); sin él `/ready` da 503.
- Sin variables nuevas.

## Aprobación
- **Estado:** 🚧 Borrador (2026-11-25), ejecutada el mismo día; falta la firma del revisor.
- **Revisores:** Osmar Reséndiz (pendiente de aprobar)
- **Decisiones para el revisor:** (1) D-1, sin nombre de quien pagó; (2) los filtros de fecha van sobre la fecha del cobro; (3) los totales cuentan solo pagos cobrados.
- [ ] Revisor 1 — fecha:
- [ ] CHECKLIST_PR.md OK
- [x] Sin conflicto con constitución
