# NV-94 — Restablecer la contraseña con código

## 1. Objetivo
Hoy `POST /auth/recuperar` manda un enlace `{APP_LINK_BASE}/restablecer#token=…` (nv-04, nv-06). Si el usuario lo abre en la computadora, cae en la página del servidor y no en la app, el mismo problema que nv-91 resolvió para la verificación. Esta spec cambia el enlace por un **código numérico de 6 dígitos** que el usuario escribe en la app junto con su correo y la contraseña nueva. **Origen**: pedido del revisor (2026-11-26). **Depende de**: NV-04 (auth y `tokens_un_uso`), NV-06 (correo), NV-91 (código de 6 dígitos).

## 2. Alcance / No-alcance
- **Dentro:** código de restablecer (generación, HMAC, vencimiento, intentos, bloqueo, tope diario), plantilla del correo, cuerpo nuevo de `POST /auth/restablecer`, pruebas y documentos.
- **Fuera:** la descarga de «mis datos» sigue con enlace (se abre en el navegador). La pantalla en la app móvil va en el repositorio de la app.

## 3. Contratos API
| Método | Ruta | Auth | Rate | Códigos |
|---|---|---|---|---|
| POST | `/api/v1/auth/recuperar` → `{correo}`; código de 6 dígitos de 15 min; siempre 202; 60 s entre códigos y 5 por 24 h | no | 10/min por IP | 202/400/422/429 |
| POST | `/api/v1/auth/restablecer` → `{correo, codigo, password_nuevo, dispositivo?}` → tokens; un solo uso; 5 intentos | código del correo | 10/min por IP | 200/400/422/423/429 |

- `recuperar` responde 202 igual exista o no el correo, y también cuando no manda nada por el tope (60 s o 5 en 24 h): no revela si la cuenta existe.
- **400** «el código no es válido o venció; pide uno nuevo»: código incorrecto, vencido, usado, inexistente, **o correo sin cuenta o que no puede entrar** (mismo mensaje, para no revelar qué correos tienen cuenta). **423**: el código actual llegó a 5 intentos fallidos; solo sirve pedir otro. **422**: forma inválida (correo, código de 6 dígitos o contraseña débil), antes de gastar un intento.
- Al restablecer: cambia la contraseña, cierra las demás sesiones, borra `login_fallos:{correo}` y abre una sesión nueva (igual que antes).

## 4. Modelo de datos
- `tokens_un_uso` (tipo `recuperar_password`): mismo esquema que nv-91 (`token_hash` = HMAC del id + código, `intentos`, `expira_en` = creación + 15 min). Sin índices ni variables nuevas.
- Los enlaces de restablecer ya enviados no coinciden con ningún HMAC: vencen solos por `ttl_token`.

## 5. Seguridad
- Código con `crypto/rand`, guardado solo como HMAC; comparación de tiempo constante; intentos sumados de forma atómica **antes** de comparar; del 6.º en adelante = 423.
- **D-1 — revisa la D-1 de nv-91.** nv-91 dejó restablecer con enlace porque la ruta es pública y adivinar el código da la cuenta completa. Ahora el tope diario de restablecer baja a **5 códigos** (10 en verificación): 5 × 5 = 25 intentos diarios por cuenta sobre 10⁶ combinaciones (2,5 × 10⁻⁵ por cuenta y día), más 10/min por IP en las dos rutas. El revisor acepta ese riesgo a cambio de que funcione al abrir el correo en otro equipo.
- **Un solo uso real:** `UseToken` falla si otra petición marcó el código primero; dos restablecer simultáneos con el código correcto no pasan los dos.
- Pedir otro código invalida el anterior. El código nunca va a logs; `mailer.Log` sigue registrando solo tipo y destinatario enmascarado.

## 6. Tests / Gates
- **Unit:** `internal/auth/dto_test.go` (`TestResetRequest_CorreoYCodigo`: sin correo, correo inválido, sin código, código corto o con letra, contraseña débil = 422); `internal/auth/codigo_test.go` (`TestCodesPerDay`); `internal/platform/mailer/smtp_test.go` (`TestRender_PasswordResetShowsTheCodeWithoutLink`).
- **Integration:** `tests/integration/auth_test.go` (`TestAuth_RecoverThenReset`: código malo y correo sin cuenta = mismo 400, caso feliz con sesión nueva, sesiones viejas cerradas, código de un solo uso; `TestAuth_ResetCodeLocksAfterFiveAttempts`: 423 aunque llegue el correcto); `tests/integration/auth_service_test.go` (`TestAuthService_ExpiredResetCodeIs400`, `TestAuthService_ResetCodesPerDay`); `tests/integration/mailer_test.go` (Mailpit trae el código y ningún enlace).
- **Security:** `tests/security/auth_security_test.go` (`TestAuthSecurity_NoTokensOrPasswordsInLogs` con el código y la contraseña nueva).
- **Contract:** `auth/restablecer_200` con el cuerpo nuevo (misma respuesta).

## 7. Plan de entrega
- Sin índices ni variables nuevas. Al desplegar, los enlaces de restablecer ya enviados dejan de servir: el usuario pide un código desde la app.
- La app móvil cambia «Nueva contraseña» desde el enlace por una pantalla con el código de 6 dígitos, la contraseña nueva y «Reenviar código» (`POST /auth/recuperar` otra vez).

## Aprobación
- **Estado:** 🚧 Borrador (2026-11-26), ejecutada el mismo día; falta la firma del revisor.
- **Revisores:** Osmar Reséndiz (pendiente de aprobar)
- **Decisiones para el revisor:** (1) 6 dígitos, 15 min, 5 intentos; (2) D-1, tope de 5 códigos al día y aceptar el riesgo de una ruta pública con código; (3) correo sin cuenta = mismo 400.
- [ ] Revisor 1 — fecha:
- [ ] CHECKLIST_PR.md OK
- [x] Sin conflicto con constitución
