# plan.md — NV-42 Evaluación de imágenes y texto

1. Mapeo
- Crear: `internal/moderacion/` (`moderacion.go`, `reglas.go`, `texto.go`, `rekognition.go` y pruebas), `internal/testutil/rekognitiontest/`.
- Modificar: `infra/aws-dev/main.tf` (política del rol), `infra/aws-dev/iam/wherever-dev-boundary.json`, `infra/aws-dev/iam/wherever-dev-deployer-policy.json`, `infra/aws-dev/README.md`, `go.mod` (AWS SDK v2), `Memoria.md`.
- Sin cambios: `cmd/api` (sin consumidor hasta nv-45), `docs/endpoints.md` (sin rutas), `docs/aviso-privacidad.md` (lo cambia nv-45). La guía de operación va en `infra/aws-dev/README.md` en lugar de un `docs/moderacion.md` aparte.

2. Guardarraíles
- Proveedores probados con dobles locales de su protocolo, sin secretos en Git, sin contenido en logs.
- Falla cerrada: sin resultado, no se publica.

3. Secuencia
1. Preparación: drift y costo en la spec.
2. Implementación.
3. Cierre.

4. Verificación
- Local: `go build`, `go vet`, Unit, `golangci-lint`, `critic`, `check_endpoints`, `check_sdd`. Integration, Security, Database, Contract, cobertura y Semgrep en el CI del PR.
