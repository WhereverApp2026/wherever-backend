# NV-58 — La Vaca: gastos compartidos del grupo

## Contexto
Registra lo que cada integrante pagó durante la salida y calcula quién le debe a quién con el menor número de transferencias, cuadrando al centavo. Wherever solo calcula: el dinero se mueve por el banco de cada persona, nunca por la app. **Origen**: hoja «J4 La Vaca» del Excel (roles C4, backend B4; pantallas P-26 y resumen de Encuentro Amigos / Pareja). **Depende de**: NV-37 (grupos e integrantes), NV-30 (planes), NV-20 (negocios, solo para `negocio_id`). **Relacionada**: NV-47 (captura sin conexión por `/sync/pendientes`).

## Requisitos
1. `POST /grupos/{id}/gastos` registra un gasto (monto, concepto, quién pagó y entre quiénes); valida que todos sean integrantes y que la división personalizada sume el total; responde el gasto y el corte actualizado.
2. `GET /grupos/{id}/gastos` lista los gastos con filtros `concepto` y `pagado_por`, por cursor, con el total del grupo.
3. `PATCH /gastos/{id}` corrige un gasto y `DELETE /gastos/{id}` lo borra (lógico); ambos recalculan el corte.
4. `GET /grupos/{id}/vaca` devuelve el corte: total, cuánto puso y cuánto le toca a cada quien, y las transferencias mínimas.
5. `PATCH /vaca/deudas/{id}` marca una deuda como pagada con referencia opcional.
6. `GET /vaca/mis-deudas` muestra lo que debo y lo que me deben en todos mis grupos.
7. Un gasto capturado sin conexión no se duplica al reenviarse (`id_cliente`).

## Diseño
- **Paquete `internal/vaca`:** modelo, repositorio, service, handler, DTO, cálculo del corte (`corte.go`, puro, sin base) y `Privacy` (Purger y Exporter de nv-05). Consume el repositorio de grupos de nv-37 por una interfaz `Members` (integrantes y organizador de un grupo), no su paquete completo.
- **Integrante:** el organizador o un integrante con estado `confirmado` en `grupos.integrantes` (nv-37). Invitado o declinado no cuenta. Grupo ajeno o inexistente = 404 igual.
- **Colección `gastos`** (J4 §1): `grupo_id`, `plan_id?`, `concepto` (`comida`, `bebidas`, `transporte`, `entrada`, `otro`), `descripcion?` (≤ 60), `monto_centavos` (1 a 10 000 000), `pagado_por`, `dividir_entre[]` (2 a 12, sin repetidos; por defecto todos los integrantes), `division` (`igual`, `personalizada`), `montos_personalizados[{usuario_id, monto_centavos}]` (suman exacto el total, uno por persona de `dividir_entre`), `negocio_id?`, `id_cliente` (UUID), `creado_por`, `eliminado_en?`, `version`, `creado_en`, `actualizado_en`. Índices `ix_gastos_grupo`, `ix_gastos_pagador` y `ux_id_cliente` (único `creado_por + id_cliente`).
- **Colección `deudas_vaca`** (J4 §2): `grupo_id`, `deudor_id`, `acreedor_id`, `monto_centavos`, `estado` (`pendiente`, `pagada`), `referencia?` (≤ 60), `pagada_en?`, `version_corte`, `creado_en`, `actualizado_en`. Índice único `ux_deuda_corte` (`grupo_id + deudor_id + acreedor_id + version_corte`), `ix_deudas_grupo`, `ix_mis_deudas` (deudor) e `ix_mis_deudas_acreedor`.
- **Cálculo (`corte.go`):** saldo de cada persona = lo que pagó − lo que le toca + lo que ya transfirió en deudas `pagada` − lo que ya recibió en deudas `pagada`. Con división igual, el residuo de la división entera se asigna a quien pagó si está en `dividir_entre`; si no, al primero de `dividir_entre` por id. Luego, el que más debe le paga al que más le deben hasta que uno queda en cero, y se repite; empates por id para que el resultado sea determinista. Todo en centavos enteros; la suma de saldos siempre es 0.
- **Recálculo:** cada alta, cambio o baja de gasto, y cada deuda marcada como pagada, vuelve a calcular dentro de `lock:vaca:{grupo_id}` (TTL 10 s; Redis caído = 503, fail-closed como todo candado). Borra las deudas `pendiente` del corte anterior, escribe las nuevas con `version_corte + 1` y conserva las `pagada`. Costo en Upstash: 2 comandos por escritura (SET NX + EVALSHA que borra solo si el candado sigue siendo propio); espera hasta 3 s y luego 503.
- **Cambiar o borrar un gasto con deudas pagadas:** si el grupo tiene deudas `pagada`, el cambio responde 409 `deudas_pagadas` con la lista, salvo que el cuerpo traiga `confirmar: true`. Con la confirmación se aplica: las pagadas se conservan como transferencias hechas y el corte nuevo puede generar una deuda de regreso.
- **Quién edita:** quien capturó el gasto o el organizador; `If-Match: "<version>"` obligatorio en `PATCH` y `DELETE` de gastos (AGENTS §5). Cualquier otro integrante = 403; ajeno = 404.
- **Deudas:** solo el deudor o el acreedor marcan `pagada`; ya pagada = 409. Sin `If-Match`: es una transición con filtro condicional `estado: pendiente`, como nv-36. No hay "desmarcar" en esta spec.
- **Sin conexión:** `POST` con un `id_cliente` ya usado por la misma persona responde 200 con el gasto existente (no 201 ni 409), para que reenviar sea seguro. La entrada por `/sync/pendientes` es de nv-47: allí el tipo `gasto` llamará al mismo service.
- **Notificaciones:** "gasto agregado" no se envía mientras nv-40 y nv-41 estén pospuestas.
- **ARCO:** borrar la cuenta no borra gastos ni deudas, porque son datos del grupo; el `Purger` cambia el nombre que se muestra por «Cuenta eliminada» (la respuesta resuelve nombres desde `usuarios` y la cuenta ya no está). "Borrar historial" no toca la Vaca. La exportación incluye `gastos` que pagó o capturó y `deudas_vaca` donde es deudor o acreedor.
- **Límites de uso:** sin política nueva de Redis; aplica `Writes` (10 concurrentes por usuario). Hasta 500 gastos vivos por grupo (501.º = 422).
- **Variables:** ninguna nueva; nada para Parameter Store.

## Contratos
| Método | Ruta | Auth | Respuestas |
|---|---|---|---|
| POST | `/api/v1/grupos/{id}/gastos` → `{monto_centavos, concepto, descripcion?, pagado_por, dividir_entre[]?, division, montos_personalizados[]?, plan_id?, negocio_id?, id_cliente}`; 201 `{gasto, corte}` (200 si `id_cliente` repetido) | JWT integrante | 200, 201, 400, 401, 403, 404, 422 |
| GET | `/api/v1/grupos/{id}/gastos?concepto=&pagado_por=&cursor=` → `{items[], siguiente, total_centavos}` | JWT integrante | 200, 401, 403, 404, 422 |
| PATCH | `/api/v1/gastos/{id}` parcial + `confirmar?`, `If-Match` | JWT quien capturó u organizador | 200, 400, 401, 403, 404, 409, 422 |
| DELETE | `/api/v1/gastos/{id}?confirmar=`, `If-Match` | JWT quien capturó u organizador | 204, 400, 401, 403, 404, 409 |
| GET | `/api/v1/grupos/{id}/vaca` → `{total_centavos, por_persona[{usuario_id, nombre, puso, le_toca, saldo}], deudas[]}` | JWT integrante | 200, 401, 403, 404 |
| PATCH | `/api/v1/vaca/deudas/{id}` → `{estado: "pagada", referencia?}` | JWT deudor o acreedor | 200, 400, 401, 403, 404, 409, 422 |
| GET | `/api/v1/vaca/mis-deudas?estado=` → `{debo[], me_deben[], total_debo, total_me_deben}` | JWT | 200, 401, 403, 422 |

403 = cuenta en eliminación pendiente (`usuarios.RequireActive`) o integrante sin permiso de edición. Sin `If-Match` = 400; versión vieja = 409 (como `PATCH /usuarios/yo`). Filas para `docs/endpoints.md` al implementar.

**Drift corregido contra el Excel:**
- J4 no pide `version` ni `If-Match`; se agregan en `PATCH`/`DELETE` de gastos (AGENTS §5).
- J4 dice "409 si ya hay deudas pagadas (avisa)" sin salida; se define `confirmar: true`.
- J4 no dice qué pasa con el centavo sobrante si quien pagó no está en `dividir_entre`; se define.
- `id_cliente` repetido: J4 dice "sin duplicar"; se responde 200 con el existente.
- Integrante = `confirmado` u organizador (nv-37 usa `invitado|confirmado|declinado`).
- La ruta de sincronización es de nv-47, no de esta spec.

## Tests
- Unit `internal/vaca/corte_test.go`: tabla de casos (división igual con residuo, personalizada, quien pagó fuera de `dividir_entre`, deudas pagadas que invierten el sentido, 12 personas); propiedad: la suma de saldos es 0 y no hay más de n − 1 transferencias. DTOs y entradas inválidas (monto 0, suma personalizada distinta, repetidos, concepto fuera de lista).
- `tests/integration/vaca_test.go` con MongoDB y Redis reales: alta con corte, `id_cliente` repetido (200 y un solo documento), no integrante en `pagado_por` (422), ajeno (404), lista con filtros y cursor, editar sin `If-Match` (400) y con versión vieja (409), editar con deudas pagadas (409 y luego `confirmar`), borrar, marcar pagada y ya pagada (409), tercero marcando (403), mis deudas en dos grupos, borrar cuenta y exportar.
- `TestVaca_Concurrencia_UnSoloCorte`: 10 altas simultáneas en un grupo dejan un solo juego de deudas `pendiente` (una `version_corte`) que cuadra.
- `tests/security/vaca_security_test.go`: las siete rutas con 401 sin token y 403 con la cuenta en eliminación pendiente; Redis caído en una escritura = 503.
- `tests/database/indexes_test.go`: índices de `gastos` y `deudas_vaca`. `tests/contract`: `gastos: []` y `deudas_vaca: []` en la exportación.

## Criterios
- El corte cuadra al centavo y es igual para todos los integrantes.
- Ningún reenvío del teléfono duplica un gasto.

## Límites
- Wherever no mueve dinero de la Vaca: sin Stripe ni SPEI aquí (los pagos con Stripe son de nv-60 y solo para reservas).
- Sin fotos de ticket, sin monedas distintas a MXN, sin desmarcar deudas pagadas.
- Sin notificaciones hasta nv-40 y nv-41.

## Aprobación y Control de Cambios
- **Estado:** 🚧 Borrador con evidencia (2026-11-13), ola 8; implementado en el PR de nv-58 contra `main`
- **Revisores:** Osmar Reséndiz (pendiente de aprobar)
- **Fecha:** —
- **Decisiones tomadas al ejecutar (pendientes de firma):** `confirmar: true` para editar con deudas pagadas; candado `lock:vaca:{grupo}` en Redis (2 comandos por escritura) frente a un contador en Mongo; residuo al primero por id si quien pagó no participa; la Vaca se conserva al borrar la cuenta (nombre «Cuenta eliminada»); tope de 500 gastos por grupo.
- **Detalle:** la evidencia irá en `specs/nv-58/checklists/gates.md` + `Memoria.md`.
