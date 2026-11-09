# plan.md — NV-44 Infraestructura de imágenes: S3 y CloudFront

1. Mapeo
- Crear: `internal/imagenes/*`, `infra/aws-dev/imagenes.tf`, pruebas en `tests/`.
- Modificar: boundary y deployer (S3), `internal/privacidad/purger.go`, `infra/aws-dev/outputs.tf`, `docs/endpoints.md`, `Memoria.md`.
- Sin cambios: lo que esta spec no nombra.

2. Guardarraíles
- Servicios reales en las pruebas, sin secretos en Git, sin datos personales en logs, roles validados en cada endpoint y cada ruta nueva en `docs/endpoints.md`.
- Bucket sin acceso público; sin ACL.

3. Secuencia
1. Preparación
2. Implementación
3. Cierre

4. Verificación
- `go build`, `go vet`, cinco suites, `critic`, `check_endpoints`, `check_sdd`, cobertura ≥ 45 %, Semgrep en 0.
