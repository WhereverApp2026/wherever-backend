# plan.md — NV-45 Subida, validación y publicación de imágenes

1. Mapeo
- Crear: `internal/imagenes` (modelo, repositorio, almacén S3, proceso, service, revisión, handler, DTO, privacidad), `internal/testutil/imagenestest`, `tests/integration/imagenes_test.go`, `tests/security/imagenes_security_test.go`.
- Modificar: `cmd/api/main.go`, `internal/platform/config`, `internal/indexes`, `internal/testutil/authtest`, `scripts/routes`, `configs/.env.example`, `infra/aws-dev/main.tf` (`hop_limit` 2), `docs/endpoints.md`, `docs/rate-limit-matrix.md`, `docs/aviso-privacidad.md`, contrato de exportación ARCO.
- Sin cambios: lo que esta spec no nombra.

2. Guardarraíles
- S3 y Rekognition con dobles (interfaces externas); MongoDB y Redis reales. Sin secretos en Git, sin datos personales en logs, dueño validado en el service y cada ruta en `docs/endpoints.md`.
- Bucket sin acceso público; sin ACL; solo `i/` sale por CloudFront.

3. Secuencia
1. Preparación
2. Implementación
3. Cierre

4. Verificación
- Local: `go build`, `go vet`, Unit, `golangci-lint`, `critic`, `check_endpoints`, `check_sdd`. CI: Integration, Security, Database, Contract, cobertura ≥ 45 %, Semgrep en 0.
