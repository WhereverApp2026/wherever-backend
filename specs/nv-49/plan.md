# plan.md — NV-49 Cifrar los datos sensibles en la base

1. Mapeo
- Crear: `internal/usuarios/cifrado.go` (capa del repositorio y `Reseal`); `scripts/migrar-cifrado/main.go`; `tests/integration/cifrado_test.go`.
- Modificar: `internal/usuarios/repository.go` (`BirthDateStored`, `Adult`, `cipher`, cifrar en `Create` y `apply`, descifrar al leer); `cmd/api/main.go` (`fieldCipher`, también en `startVotes`); `internal/testutil/authtest/authtest.go` (`testCipher`); `Makefile` (`migrar-cifrado`); `Memoria.md`.
- Sin cambios: `account`, `recomendaciones`, `asistente` y la exportación (leen por el repositorio).

2. Guardarraíles
- Servicios reales en las pruebas, sin secretos en Git, sin datos personales en logs.
- Un valor que no abre falla cerrado; nunca se devuelve el texto cifrado como dato.

3. Secuencia
1. Preparación: corregir la spec (el ánimo no se guarda; el comando va en `scripts/`).
2. Implementación: capa de cifrado en el repositorio y migración idempotente.
3. Cierre: pruebas, documentos, `Memoria.md`, PR.

4. Verificación
- `go build`, `go vet`, Unit y `golangci-lint` en local; Integration, Security, Database y Contract en el CI del PR; `critic`, `check_endpoints`, `check_sdd`, cobertura ≥ 45 %, Semgrep en 0.
