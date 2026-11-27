# plan.md — NV-94 Restablecer la contraseña con código

1. Mapeo
- `internal/auth` (service, DTO, handler, repositorio, tope por tipo), `internal/platform/mailer` (plantilla y envío sin enlace).
- `docs/endpoints.md`, `internal/http/openapi.json`, `AGENTS.md` §7 (regla del fragmento), `Memoria.md`.

2. Guardarraíles
- Reusar el código de nv-91; código solo como HMAC; intentos atómicos; sin índices ni variables nuevas; mocks solo de interfaces externas.

3. Secuencia
1. Spec y filas de `docs/endpoints.md`.
2. Service: `checkCode` y `sendCode`/`codeWait` por tipo; `ResetPassword` por correo y código; `UseToken` de un solo uso real.
3. DTO, handler y plantilla.
4. Pruebas, OpenAPI, documentos y `Memoria.md`.

4. Verificación
- build, vet, Unit y lint en local (Docker `golang:1.27.3`); Integration, Security, Database y Contract en el CI del PR.
