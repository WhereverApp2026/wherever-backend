# plan.md — NV-26 Cliente de Bedrock y salida validada

1. Mapeo
- Crear: `internal/ia/*`, `docs/ia.md`, `tests/ia/*`.
- Modificar: `internal/platform/config`, `.env.example`, `infra/aws-dev/iam/wherever-dev-boundary.json` y el rol de la instancia (permiso de Bedrock), `docs/rate-limit-matrix.md`, `Memoria.md`.
- Sin cambios: lo que esta spec no nombra.

2. Guardarraíles
- Servicios reales en las pruebas, sin secretos en Git, sin datos personales en logs, roles validados en cada endpoint y cada ruta nueva en `docs/endpoints.md`.
- Instrucciones y entrada del usuario separadas siempre.

3. Secuencia
1. Preparación
2. Implementación
3. Cierre

4. Verificación
- `go build`, `go vet`, cinco suites, `critic`, `check_endpoints`, `check_sdd`, cobertura ≥ 45 %, Semgrep en 0.
