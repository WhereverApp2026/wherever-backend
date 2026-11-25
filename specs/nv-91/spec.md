# NV-91 — Verificación de correo con código

## 1. Objetivo
Hoy el correo de verificación trae un enlace `{APP_LINK_BASE}/verificar#token=…` (nv-04, nv-06). Si el usuario lo abre en la computadora, cae en la página del servidor y no en la app. Esta spec cambia el enlace por un **código numérico de 6 dígitos** que el usuario escribe en la app. **Origen**: prueba del alta en el servidor (2026-11-25). **Depende de**: NV-04 (auth y `tokens_un_uso`), NV-05 (registro y cambio de correo), NV-06 (correo).

## 2. Alcance / No-alcance
- **Dentro:** código de verificación (generación, hash, vencimiento, intentos, bloqueo), plantilla del correo, `POST /auth/verificar-correo` con el código, límites de `reenviar-verificacion`, pruebas y documentos.
- **Fuera:** restablecer la contraseña y la descarga de «mis datos» siguen con enlace (ver §5, decisión D-1). La pantalla del código en la app móvil va en el repositorio de la app.

## 3. Contratos API
| Método | Ruta | Auth | Rate | Códigos |
|---|---|---|---|---|
| POST | `/api/v1/auth/verificar-correo` → `{codigo}` (6 dígitos) → `{correo_verificado: true}`; código de 15 min, un solo uso, 5 intentos | JWT | 10/min por IP | 200/400/401/403/422/423/429 |
| POST | `/api/v1/auth/reenviar-verificacion` → manda un código nuevo e invalida el anterior (1 cada 60 s y 10 por día por usuario, `Retry-After`) | JWT | 10/min por IP | 202/401/403/429 |

- `verificar-correo` deja de ser pública: el código solo identifica al usuario junto con su sesión. El registro ya devuelve tokens (nv-05), así que la app tiene la sesión al pedir el código.
- Con el correo ya verificado, `verificar-correo` responde 200 sin revisar el código (idempotente) y `reenviar-verificacion` 202 sin mandar nada.
- **400**: código incorrecto, vencido, ya usado o inexistente («el código no es válido o venció; pide uno nuevo»). **423**: el código actual llegó a 5 intentos fallidos; solo sirve pedir otro. **429**: límite por IP, o reenvío antes de 60 s o tras 10 códigos en 24 h.
- **401** sin token; **403** con la cuenta en eliminación o no activa (`RequireActive`).

## 4. Modelo de datos
- `tokens_un_uso` (tipo `verificar_correo`): `token_hash` = HMAC-SHA256 del código; nuevo campo `intentos` (entero, 0 al crear). `expira_en` = creación + 15 min. Sin índices nuevos: la búsqueda usa `ix_usuario_tipo` (`usuario_id`, `tipo`, `creado_en`) y el vencimiento `ttl_token`.
- El HMAC usa una llave derivada de `JWT_SECRET` (`HMAC(JWT_SECRET, "wherever/codigo-correo/v1")`) y firma `id del documento + código`. El id evita choques en `ux_token_hash` (dos usuarios o dos códigos con los mismos dígitos). Sin variables nuevas.
- Los enlaces viejos de verificación que sigan en la base no coinciden con ningún HMAC: vencen solos por `ttl_token`.

## 5. Seguridad
- Código con `crypto/rand` (uniforme en 000000–999999). Nunca en claro en la base ni en logs; el `mailer.Log` sigue registrando solo tipo y destinatario enmascarado.
- **Intentos:** cada intento suma 1 a `intentos` de forma atómica **antes** de comparar (dos peticiones a la vez no se saltan el tope); la comparación es de tiempo constante. Del 6.º intento en adelante = 423 aunque el código sea correcto.
- **Un solo código vivo:** pedir otro marca como usados los anteriores. Usar el correcto lo marca como usado.
- **Fuerza bruta:** 5 intentos por código × 10 códigos al día = 50 intentos diarios por cuenta sobre 10⁶ combinaciones, más el límite por IP. Quien intenta adivinar necesita la sesión de la cuenta, porque la ruta exige JWT.
- **Reenvío:** llave propia `rate:auth_reenviar:{ip}` (10/min), 60 s entre códigos y 10 códigos por 24 h por usuario (cuenta en `tokens_un_uso`; el registro y el cambio de correo no cuentan contra este tope).
- **D-1 — restablecer la contraseña sigue con enlace.** Porqué: `/auth/restablecer` es pública y abre una sesión; con un código de 6 dígitos el tope de intentos tendría que ir por correo y sin sesión, y adivinarlo daría la cuenta completa (no solo marcar un correo). El enlace de 256 bits no tiene ese riesgo. Si la app necesita lo mismo, va en otra spec con su propio análisis (código más largo o enlace a la app con App Links).

## 6. Tests / Gates
- **Unit:** `internal/auth/codigo_test.go` (6 dígitos, HMAC distinto por documento y por código, comparación); `internal/auth/dto_test.go` (validación del `codigo`: vacío, 5 o 7 dígitos, letras = 422); `internal/platform/mailer/smtp_test.go` (la plantilla de verificación muestra el código y no un enlace).
- **Integration:** `tests/integration/auth_test.go` caso feliz (registro → correo → código → `correo_verificado`), código malo, vencido, 6.º intento = 423, reenvío invalida el anterior, tope diario; `tests/integration/mailer_test.go` el correo de Mailpit trae el código de 6 dígitos y ningún `#token=`.
- **Security:** `tests/security/auth_security_test.go` 401 sin token, 403 con la cuenta en eliminación, el código de otro usuario no sirve, el hash guardado no es el código.
- **Contract:** `auth/verificar_correo_200` y `auth/verificar_correo_400` con el cuerpo nuevo.

## 7. Plan de entrega
- Sin índices ni variables nuevas. Al desplegar, los enlaces de verificación ya enviados dejan de servir: el usuario pide un código desde la app.
- La app móvil cambia la pantalla «Revisa tu correo» por un campo de 6 dígitos y el botón «Reenviar código» (`POST /auth/reenviar-verificacion`).

## Aprobación
- **Estado:** 🚧 Borrador (2026-11-25), ejecutada el mismo día; falta la firma del revisor.
- **Revisores:** Osmar Reséndiz (pendiente de aprobar)
- **Decisiones para el revisor:** (1) 6 dígitos, 15 min y 5 intentos; (2) `verificar-correo` pasa a exigir JWT; (3) D-1, restablecer sigue con enlace.
- [ ] Revisor 1 — fecha:
- [ ] CHECKLIST_PR.md OK
- [x] Sin conflicto con constitución
