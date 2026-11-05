# plan.md — NV-15 Correcciones de la auditoría

1. Mapeo
- Crear: `internal/platform/config/internals_test.go`, `tests/integration/correcciones_test.go`.
- Modificar: `internal/platform/config/config.go`, `configs/.env.example`, `internal/http/app.go`, `internal/privacidad/job.go`, `cmd/api/main.go`, `internal/usuarios/repository.go`, `internal/usuarios/account/{dto,service}.go`, `internal/http/openapi.json`, `docs/endpoints.md`, `infra/aws-dev/README.md`, `tests/contract/`, `Memoria.md`.
- Sin cambios: `ci-pr.yml`, `cd-dev.yml`, Terraform.

2. Guardarraíles
- Prod sigue sin `detalle` ni sondas, con o sin el indicador.
- Dev y las pruebas de staging no cambian: el indicador nace apagado.
- Retirar el consentimiento no falla si la cuenta no tiene gustos.

3. Pruebas
- Locales: `go build`, `go vet`, cinco suites, lint, Semgrep, `critic`, `check_sdd`, `check_endpoints`.
- Reales: sondas en 404 en el servidor público y trabajo corriendo a las 21:00.

4. Secuencia
1. Configuración y sus pruebas.
2. Hora del trabajo.
3. Consentimiento y pruebas.
4. Datos internos y pruebas.
5. Contrato y documentos.
6. Cargar `JOB_HOUR` y `HIDE_INTERNALS` en Parameter Store antes de fusionar: la API los toma al instalar la imagen nueva.
7. PR, fusión y comprobación en el servidor.

5. Verificación
- `curl -s -o /dev/null -w '%{http_code}' $API_URL/api/v1/probe/error → 404`
- `journalctl`/logs de la API a las 21:00: corrió el trabajo.
