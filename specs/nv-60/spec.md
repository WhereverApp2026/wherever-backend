# NV-60 — Pagos con Stripe: anticipo, webhook y depósitos a negocios

## Contexto
Cobra el anticipo de una reserva con Stripe, confirma la reserva cuando Stripe avisa por un webhook firmado, devuelve el dinero al cancelar y deposita a cada negocio lo suyo menos la comisión. La tarjeta la captura el componente de Stripe en la app; el backend nunca recibe el número ni el CVC. **Origen**: parte de pagos de la hoja «J5 Reservas y pagos» (P-22 Pago, P-23 Confirmada), la parte de cobros de «A5 Negocios aliados» (`cobros`, `cuenta_deposito`, `liquidaciones`, corte semanal) y lo que promete el aviso de privacidad (rama `aviso-privacidad-integral`, aún no en `main`). **Depende de**: NV-59 (reservas), NV-55 (negocios del comerciante). **Requiere**: la enmienda de la constitución §3 de esta spec.

## Requisitos
1. `POST /reservas/{id}/pago` (con `Idempotency-Key`) crea o reutiliza el PaymentIntent del anticipo y devuelve el `client_secret` para la PaymentSheet; 410 si el apartado de 10 minutos ya venció.
2. `POST /webhooks/stripe` verifica `Stripe-Signature`, aplica cada evento una sola vez y confirma la reserva al aprobarse el pago.
3. `GET /pagos/{id}/estado` da el estado al volver de 3D Secure, sin volver a cobrar.
4. Cancelar una reserva pagada (nv-59) pide el reembolso a Stripe según la política.
5. El negocio conecta su cuenta para depósitos, ve cuánto recibe de cada anticipo, su saldo y sus liquidaciones semanales.
6. Wherever solo guarda monto, fecha, estado, concepto, marca y últimos 4 dígitos de la tarjeta; los registros de pago se conservan 5 años.

## Diseño
- **Paquete `internal/pagos`:** modelo, repositorio, service, handler, DTO, `webhook.go`, `liquidacion.go` y `Privacy`. Implementa el puerto `reservas.Refunder` de nv-59 y el `reservas.Payments` que quita el 422 `anticipo_no_disponible`.
- **Interfaz externa `StripeAPI`** (única frontera con Stripe): `CreatePaymentIntent`, `GetPaymentIntent`, `CreateRefund`, `CreateAccount`, `CreateAccountLink`, `CreateTransfer`, `CreateExternalAccount`. La implementación usa `github.com/stripe/stripe-go` (versión fijada) con la URL base configurable; nada fuera de `internal/pagos` importa Stripe.
- **Tarjeta fuera del backend:** la app usa la PaymentSheet de Stripe con el `client_secret`; ningún DTO tiene campos de tarjeta y todos rechazan campos desconocidos (400), así que un cuerpo con `numero` o `cvc` nunca se guarda. Al aprobarse el pago, `tarjeta {marca, ultimos4}` se toma de `latest_charge.payment_method_details.card` del PaymentIntent.
- **Colección `pagos`** (J5 §2): `reserva_id`, `usuario_id`, `negocio_id`, `concepto` (`anticipo_reserva`), `payment_intent_id`, `monto_centavos`, `comision_centavos`, `moneda` (`mxn`), `estado` (`requiere_pago`, `procesando`, `aprobado`, `rechazado`, `reembolsado_parcial`, `reembolsado`), `motivo_rechazo?`, `tarjeta? {marca, ultimos4}`, `reembolsos[{refund_id, monto_centavos, fecha, motivo}]`, `idempotency_key`, `eventos_procesados[]` (últimos 20 ids), `conservar_hasta` (creado + 5 años), `creado_en`, `actualizado_en`. Índices de J5 (`ux_payment_intent`, `ux_idempotency`, `ix_pago_reserva`, `ix_conciliacion`) más `ttl_conservar_hasta` (TTL de Mongo con `expireAfterSeconds: 0`), que borra cada pago al cumplir 5 años sin trabajo propio.
- **Crear el cobro:** solo el dueño de una reserva `bloqueada` vigente; el monto sale de `reservas.anticipo` (nunca del cliente). Si ya hay un pago `requiere_pago` o `procesando` de esa reserva, se devuelve su `client_secret` (no se crea otro). `Idempotency-Key` de la app → `idem:pago:{usuario_id}:{llave}` (TTL 24 h); a Stripe va `idempotency_key = reserva:{reserva_id}:intento:{n}`. El PaymentIntent lleva `metadata.reserva_id` y `metadata.pago_id`, y vence con el apartado: si el webhook de éxito llega después, ver abajo. Redis caído = 503.
- **Webhook (`POST /api/v1/webhooks/stripe`):** sin JWT; cuerpo crudo (sin parsear antes de verificar), firma con `webhook.ConstructEvent` y tolerancia de 5 minutos; firma mala o ausente = 400 sin detalle. Idempotencia en dos capas: `idem:stripe_evento:{event_id}` con SET NX (TTL 7 días; Stripe reintenta hasta 3 días) y `eventos_procesados` en el pago como respaldo durable. Redis caído = 503, para que Stripe reintente. Eventos:
  - `payment_intent.succeeded` → pago `aprobado` con `tarjeta`; reserva `confirmada` (o `pendiente` si el negocio confirma a mano). Si la reserva ya `expirada` y el cupo sigue libre, se confirma; si no, reembolso completo automático y la reserva queda `cancelada` con motivo `cupo_perdido`.
  - `payment_intent.payment_failed` → `rechazado` con `motivo_rechazo` (mensaje de Stripe, sin datos de tarjeta); la reserva sigue `bloqueada` hasta que venza, para reintentar.
  - `charge.refunded` → agrega el reembolso y pasa a `reembolsado_parcial` o `reembolsado`.
  - `account.updated` → actualiza `negocios.cobros.estado`.
  - Cualquier otro tipo = 200 sin hacer nada. Responde 200 en menos de 10 s; el trabajo es de una sola escritura por evento.
- **Ruta del webhook en el pipeline:** exenta de `Version` (Stripe no manda `X-App-Version` y no debe recibir 426), de compresión, de CORS y del límite de escrituras por IP; pasa por el límite global. Va en la lista de exentas de `versionado`.
- **Estado del pago:** `GET /pagos/{id}/estado` solo para el dueño; lee Mongo y, si sigue `procesando` más de 30 s, consulta `GetPaymentIntent` una vez para no depender solo del webhook.
- **Reembolso:** `Refunder.Refund(reserva, monto)` llama a `CreateRefund` con `idempotency_key = reembolso:{reserva_id}` y `reverse_transfer` cuando ya se transfirió; la comisión se devuelve en la misma proporción. El estado final llega con `charge.refunded`.
- **Depósitos a negocios (Stripe Connect):** el aviso de privacidad dice que la identidad, los datos fiscales y la cuenta del negocio se transfieren a Stripe, y A5 ya trae `stripe_account_id` y `stripe_transfer_id`: sí hace falta Connect. Propuesta: cuentas **Express** de México, cobro en la cuenta de Wherever (*separate charges and transfers*) y transferencia semanal a cada negocio, como pide A5. Así Wherever nunca guarda dinero de terceros fuera de Stripe ni hace SPEI propio, y Stripe verifica la identidad (KYC).
  - `negocios.cobros {stripe_account_id, estado: pendiente|activo|restringido, comision_pct}` y `negocios.cuenta_deposito {clabe_ultimos4, banco, titular, estado, stripe_external_account_id}` (A5). La CLABE completa va a Stripe y no se guarda.
  - `POST /negocios/{id}/cobros/conectar` crea o continúa la cuenta Express y devuelve el enlace de registro de Stripe (`AccountLink`).
  - `PUT /negocios/{id}/cuenta-deposito` valida la CLABE (18 dígitos, banco por los 3 primeros, dígito verificador) y la registra como cuenta externa en Stripe.
  - **Colección `liquidaciones`** (A5): `negocio_id`, `semana_inicio`, `total_anticipos`, `comision`, `reembolsos`, `neto`, `estado` (`pendiente`, `transferida`, `fallida`), `stripe_transfer_id?`, `conservar_hasta`, `creado_en`. Índice único `ux_liquidacion` (`negocio_id + semana_inicio`).
  - **Corte semanal:** cada lunes, en una instancia (`lock:liquidacion_semanal`, 2 comandos por semana), suma los pagos `aprobado` de reservas `asistio` o `no_asistio` de la semana anterior, resta comisión y reembolsos y hace `CreateTransfer` con `idempotency_key = liquidacion:{negocio}:{semana}`. Sin cuenta `activo`, queda `pendiente` y se reintenta la semana siguiente.
  - `GET /negocios/{id}/reservas/{rid}/cobro` → `{anticipo, comision, neto, liquidacion_semana}`; `GET /negocios/{id}/saldo` → `{disponible, en_espera, proximo_deposito}`; `GET /negocios/{id}/liquidaciones` por cursor.
  - **Comisión:** `comision_pct` por negocio (A5 dice 5 % a 10 %); el valor inicial lo decide el revisor.
- **Configuración y secretos:** `STRIPE_SECRET_KEY` y `STRIPE_WEBHOOK_SECRET` como `SecureString` en Parameter Store (`/{project}/{env}/STRIPE_SECRET_KEY`, `/{project}/{env}/STRIPE_WEBHOOK_SECRET`), leídas por constructor como las demás; `STRIPE_API_URL` solo para pruebas. Fuera de `APP_ENV=prod`, una llave `sk_live_` o `rk_live_` hace fallar el arranque; en `dev` sin llave, las rutas de pago responden 503 `pagos_no_disponibles`. `.env.example` lleva los nombres sin valores; hay que sacar `STRIPE_` de `futurePrefixes` en `internal/platform/config/config_example_test.go`. La llave publicable es de la app, no del backend.
- **Logs:** se agregan `client_secret`, `stripe-signature` y `clabe` a las llaves que oculta el logger; nunca se registra el cuerpo del webhook. `POST /reservas/{id}/pago` se agrega a `carriesTokens` (lleva `client_secret`), así no se comprime (AGENTS §6).
- **Límites de uso:** `POST /reservas/{id}/pago` 10 por hora por usuario (`rate:pagos:{usuario}`); nueva fila en `docs/rate-limit-matrix.md`.
- **Upstash:** 1 comando por evento de webhook, 2 por intento de cobro y 2 por corte semanal; con 1 000 reservas pagadas al mes son unos 6 000 comandos (1,2 % de la cuota).
- **ARCO:** borrar la cuenta no borra `pagos` ni `liquidaciones` (5 años por el Código Fiscal de la Federación, como dice el aviso); `pagos` no guarda nombre, correo ni teléfono, solo ids. "Borrar historial" no los toca. La exportación incluye `pagos` (concepto, monto, fecha, estado, marca y últimos 4) y, para el comerciante, `liquidaciones`.
- **Aviso de privacidad:** el texto de pagos de la rama `aviso-privacidad-integral` debe estar en `main` antes de cobrar; si cambia, sube `AVISO_VERSION` (nv-19).

## Contratos
| Método | Ruta | Auth | Respuestas |
|---|---|---|---|
| POST | `/api/v1/reservas/{id}/pago`, `Idempotency-Key` → `{pago_id, client_secret, monto_centavos, moneda}` | JWT dueño de la reserva | 200, 400, 401, 403, 404, 409, 410, 429, 503 |
| POST | `/api/v1/webhooks/stripe`, `Stripe-Signature` | Firma de Stripe | 200, 400, 503 |
| GET | `/api/v1/pagos/{id}/estado` → `{estado, reserva_estado}` | JWT dueño | 200, 401, 403, 404 |
| GET | `/api/v1/negocios/{id}/reservas/{rid}/cobro` | JWT dueño del negocio | 200, 401, 403, 404 |
| POST | `/api/v1/negocios/{id}/cobros/conectar` → `{url}` | JWT dueño del negocio | 200, 401, 403, 404, 503 |
| PUT | `/api/v1/negocios/{id}/cuenta-deposito` → `{clabe, titular}` | JWT dueño del negocio | 200, 400, 401, 403, 404, 409, 422, 503 |
| GET | `/api/v1/negocios/{id}/saldo` | JWT dueño del negocio | 200, 401, 403, 404 |
| GET | `/api/v1/negocios/{id}/liquidaciones?cursor=` | JWT dueño del negocio | 200, 401, 403, 404 |

`POST /reservas/{id}/cancelar` (nv-59) responde además el reembolso pedido. 409 en `cuenta-deposito` = la cuenta de Connect aún no existe. Filas para `docs/endpoints.md` al implementar.

**Drift corregido contra el Excel:**
- J5 escribe `/api/v1/webhooks/stripe`: se conserva con `/api/v1/` (v1 estricta), exenta de `Version`.
- `pagos` no tenía `tarjeta`, `concepto` ni `conservar_hasta`, que pide el aviso; se agregan.
- `eventos_procesados` sin tope crecería sin límite; se guardan los últimos 20 y Redis es la primera capa.
- J5 no dice qué pasa si el pago llega después de vencer el apartado; se define.
- A5 reparte cobros, cuenta de depósito, saldo y liquidaciones en la hoja de negocios; aquí se propone que vivan en nv-60 y no en nv-55 (nota en la carpeta compartida de la ola).
- Fuera: `GET /negocios/{id}/estado-cuenta` (PDF/CSV) y facturas CFDI con PAC (A5), que piden otra spec.

**Enmienda propuesta a la constitución (§3, versión 1.8.0):** agregar la fila «Pagos | Stripe (`stripe-go`): PaymentIntents con PaymentSheet en la app y Connect Express para depósitos; el backend nunca recibe datos de tarjeta; llaves en Parameter Store; en pruebas, un doble de la API de Stripe (interfaz externa)». En §12, «1.8.0 | fecha | Stripe entra al stack para el anticipo de reservas y los depósitos a negocios (nv-60)». El texto «fuera de alcance … pagos» de la línea 15 habla de `nv-00…nv-03` y no cambia.

## Tests
- **Doble de Stripe:** `internal/testutil/stripetest` levanta un `httptest.Server` con las rutas que usa `StripeAPI` (PaymentIntents, Refunds, Accounts, AccountLinks, Transfers, External Accounts), responde como Stripe, registra las llamadas y su `Idempotency-Key`, y firma eventos con un secreto de prueba (`stripetest.SignedEvent`). El cliente real de `stripe-go` apunta a él con `STRIPE_API_URL`. Es el único doble: Mongo y Redis son reales (constitución: mocks solo de interfaces externas).
- Unit `internal/pagos`: comisión y neto en centavos, monto del anticipo, CLABE (válida, dígito malo, banco desconocido), corte semanal, DTOs que rechazan `numero`/`cvc`, llave `sk_live_` fuera de prod.
- `tests/integration/pagos_test.go` con MongoDB, Redis y el doble: crear cobro y reutilizarlo, `Idempotency-Key` repetida (una sola llamada a Stripe), apartado vencido (410), webhook aprobado (reserva confirmada, `tarjeta` guardada sin más datos), evento repetido (se aplica una vez), rechazado, pago tardío con y sin cupo, cancelar con reembolso por política, `charge.refunded`, conectar cuenta, CLABE, corte semanal idempotente, estado al volver de 3D Secure, borrar cuenta (los pagos quedan) y exportar.
- `TestPagos_Webhook_ConcurrenteUnaVez`: el mismo evento 10 veces a la vez confirma la reserva una sola vez.
- `tests/security/pagos_security_test.go`: webhook sin firma, con firma mala, con firma vieja (> 5 min) = 400; rutas con JWT: 401 sin token, 403 con rol equivocado, 404 en reserva o negocio ajeno; ninguna respuesta ni log contiene `client_secret` fuera de su ruta; Redis caído en el webhook = 503.
- `tests/database/indexes_test.go`: índices de `pagos` (incluido el TTL) y `liquidaciones`. `tests/contract`: `pagos: []` en la exportación y el cuerpo de `POST /reservas/{id}/pago`.

## Criterios
- Ningún evento de Stripe se aplica dos veces y ningún reintento cobra dos veces.
- Ni la base ni los logs contienen el número de tarjeta, el CVC o la CLABE completa.

## Límites
- Solo MXN y solo el anticipo de reservas; sin suscripciones (plan premium), propinas ni pagos de la Vaca.
- Sin facturas CFDI ni estado de cuenta descargable.
- Sin claves de Stripe en GitHub: el CI usa solo el doble.

## Aprobación y Control de Cambios
- **Estado:** 🚧 Borrador con evidencia (2026-11-14), implementada en la rama de nv-60; sin firma.
- **Revisores:** Osmar Reséndiz (decisiones tomadas el 2026-11-14; firma pendiente)
- **Fecha:** —
- **Decisiones del revisor (2026-11-14):**
  1. Se aprueba la enmienda de la constitución §3, versión 1.8.0, con la fila de Stripe (`stripe-go`); aplicada en `.specify/memory/constitution.md` (el espejo `constitution.md` solo enlaza).
  2. Stripe Connect Express con transferencia semanal: Wherever cobra el anticipo y cada lunes transfiere a cada negocio lo suyo menos la comisión. Confirmar con Stripe México que la cuenta real lo permite queda pendiente del operador; en modo de prueba está completo.
  3. `comision_pct` inicial = 10 %, configurable por negocio (`negocios.cobros.comision_pct`).
  4. Se aceptan como estaban: cobros, cuenta de depósito, saldo y liquidaciones en nv-60 (no en nv-55); pago tardío con el cupo perdido = reembolso completo; comisión proporcional al reembolso; pagos y liquidaciones borrados por TTL a los 5 años.
- **Ajustes al implementar:**
  - `PUT /negocios/{id}/cuenta-deposito` ya existía en nv-55: no se duplica. nv-60 se conecta por `alta.Deposito`: si el negocio ya tiene cuenta de Connect, la CLABE se registra en Stripe y la cuenta pasa a `registrada`; si no, queda `sin_verificar` y se registra al conectar (`POST …/cobros/conectar` lee la CLABE cifrada del expediente). Sin 409 nuevo. La copia cifrada del expediente sigue en nv-55.
  - Sin `reverse_transfer`: con cargos y transferencias separados ese parámetro no aplica, y solo se cancela una reserva futura, así que el reembolso siempre ocurre antes del corte que la transferiría. El reembolso se anota al pedirlo para que el corte no lo pague; `charge.refunded` lo concilia.
  - Un pago `rechazado` se reintenta con el mismo PaymentIntent (no se crea otro intento).
  - El `client_secret` no se guarda: se lee de Stripe al reutilizar el cobro.
  - El corte corre como tarea del trabajo nocturno (dentro de su candado), solo los lunes; no gasta comandos propios de Redis. Incluye los pagos `aprobado` o `reembolsado_parcial` con la reserva en la semana anterior, sin importar si terminó en `asistio`, `no_asistio` o `cancelada` con reembolso parcial (lo no devuelto es del negocio).
  - Webhook: `payment_intent.processing` también se aplica (`procesando`).
- **Detalle:** evidencia en `specs/nv-60/checklists/gates.md` + `Memoria.md`.
