# plan.md — NV-91 Verificación de correo con código

1. Mapeo
- `internal/auth` (código, service, DTO, rutas), `internal/platform/mailer` (plantilla y envío sin enlace).
- `docs/endpoints.md`, `docs/rate-limit-matrix.md`, `internal/http/openapi.json`, `AGENTS.md` §7 (regla del fragmento).

2. Guardarraíles
- Código solo como HMAC; intentos atómicos; sin índices ni variables nuevas; mocks solo de interfaces externas.

3. Secuencia
1. Spec y filas de `docs/endpoints.md`.
2. Código (generación, HMAC, intentos) y repositorio.
3. Service, handler, rutas y plantilla.
4. Pruebas, documentos y `Memoria.md`.

4. Verificación
- build, vet, Unit y lint en local (Docker `golang:1.27.3`); Integration, Security, Database y Contract en el CI del PR.
