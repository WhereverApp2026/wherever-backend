# plan.md — NV-72 Entrar con Google

1. Mapeo
- Crear: `internal/auth/google.go` (`GoogleVerifier` y su implementación con `idtoken`), `internal/auth/google_service.go` (entrar, vincular y crear), `internal/usuarios/google.go` (`FindByGoogleID`, `LinkGoogle`, `NewConsumer`), el emisor de llaves locales `internal/testutil/googletest` (lo conecta `authtest`) y las pruebas de Unit, Integration, Security y Contract.
- Modificar: `internal/auth` (ruta, handler con decodificación estricta, DTO, `MethodGoogle`), `internal/usuarios` (`ErrGoogleTaken` por `ux_google_id`), `internal/usuarios/account` (el registro usa `NewConsumer`), `internal/platform/config` (`GOOGLE_CLIENT_ID`), `configs/.env.example`, `infra/aws-dev/README.md`, `go.mod`, `cmd/api` (`buildAuth`), `docs/endpoints.md` (sale de «Diferidos»), `docs/rate-limit-matrix.md`, `specs/nv-04` y `specs/nv-05` (D-1 resuelta) y `Memoria.md`. `scripts/routes` no cambia: lee las rutas del módulo.
- Sin cambios: índices (`ux_google_id` y `ux_correo` ya existen), `carriesTokens` (todo `/api/v1/auth` ya va sin comprimir) y `deploy-env.sh`.

2. Guardarraíles
- La decisión D-1 y la excepción de stack se toman antes de programar (constitución §3, regla de excepción de nv-00).
- Solo el verificador de Google es simulado; Mongo y Redis reales. Sin secretos en Git, sin `id_token`, correo ni `sub` en logs.

3. Secuencia
1. Preparación: D-1 (A o B) aprobada por el revisor y el responsable de A1; excepción de stack registrada; `GOOGLE_CLIENT_ID` en config.
2. Implementación: verificador; entrar, vincular y crear cuenta.
3. Cierre: pruebas, documentos y `Memoria.md`.

4. Verificación
- En local (Docker `golang:1.27.3`): `go build`, `go vet`, Unit, golangci-lint y `govulncheck`. En el CI del PR: Integration, Security, Database, Contract, `critic`, `check_endpoints`, `check_sdd`, cobertura ≥ 45 % y Semgrep.
