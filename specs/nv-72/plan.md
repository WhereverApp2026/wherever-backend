# plan.md — NV-72 Entrar con Google

1. Mapeo
- Crear: `internal/auth/google.go` (`GoogleVerifier` y su implementación con `idtoken`), el verificador con llaves locales en `internal/testutil/authtest` y las pruebas de las cinco suites.
- Modificar: `internal/auth` (ruta, service y DTO), `internal/usuarios` (crear desde Google y vincular `google_id`; borrar `password_hash` al vincular sin verificar), `internal/platform/config` (`GOOGLE_CLIENT_ID`), `configs/.env.example`, `go.mod`, `cmd/api` y `scripts/routes`, `docs/endpoints.md` (sale de «Diferidos»), `docs/rate-limit-matrix.md` y `Memoria.md` (D-1 y la excepción de stack).
- Sin cambios: índices (`ux_google_id` y `ux_correo` ya existen).

2. Guardarraíles
- La decisión D-1 y la excepción de stack se toman antes de programar (constitución §3, regla de excepción de nv-00).
- Solo el verificador de Google es simulado; Mongo y Redis reales. Sin secretos en Git, sin `id_token`, correo ni `sub` en logs.

3. Secuencia
1. Preparación: D-1 (A o B) aprobada por el revisor y el responsable de A1; excepción de stack registrada; `GOOGLE_CLIENT_ID` en config.
2. Implementación: verificador; entrar, vincular y crear cuenta.
3. Cierre: pruebas, documentos y `Memoria.md`.

4. Verificación
- `go build`, `go vet`, cinco suites, `govulncheck` con la dependencia nueva, `critic`, `check_endpoints`, `check_sdd`, cobertura ≥ 45 %, Semgrep en 0.
