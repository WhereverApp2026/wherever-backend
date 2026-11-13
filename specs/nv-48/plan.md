# plan.md — NV-48 Capa de cifrado de campos

1. Mapeo
- Crear: `internal/platform/cifrado/` (`cifrado.go`, `cifrado_test.go` con fuzz y benchmark).
- Modificar: `internal/platform/config/config.go` (`DATA_KEYS`) y `config_test.go`; `internal/testutil/testutil.go` (llave de pruebas); `configs/.env.example`; `Memoria.md`.
- Sin cambios: endpoints, Redis, pipeline e infraestructura (el equipo carga `/wherever/dev/DATA_KEYS`).

2. Guardarraíles
- Sin secretos en Git: la llave de pruebas solo vive en `internal/testutil`; `.env.example` va vacío.
- Los errores nunca llevan el valor.

3. Secuencia
1. Preparación: corregir la spec (el ánimo no se guarda).
2. Implementación: llavero, formato, dato asociado y rotación.
3. Cierre: pruebas, documentos, `Memoria.md`, PR.

4. Verificación
- `go build`, `go vet`, Unit y `golangci-lint` en local; suites completas en el CI del PR; `critic`, `check_endpoints`, `check_sdd`, cobertura ≥ 45 %, Semgrep en 0.
