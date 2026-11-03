# plan.md — NV-05 Usuarios, perfil, gustos y privacidad (A1)

1. Mapeo
- Crear: `internal/usuarios/*`, `internal/ciudades/*`, `internal/privacidad/{purger.go,exporter.go,job.go}`, `scripts/importar_ciudades.go` (o `cmd/importar-ciudades`), `tests/{integration,security,contract}/usuarios_*_test.go`, índices de `usuarios` y `ciudades`.
- Modificar: `internal/usuarios/repository` (de `nv-04`), `internal/shared/validation/registry.go` (política de contraseña, mayoría de edad, E.164), `internal/platform/config` (`PHONE_HMAC_KEY`, `ARCO_DIAS_GRACIA`), `.env.example`, `docs/endpoints.md`, `docs/rate-limit-matrix.md`, `docs/asvs-l2-checklist.md`.

2. Guardarraíles
- Respuestas sin `password_hash`, `telefono_hash` ni `google_id`.
- `version` + `If-Match` en `PATCH`/`PUT`; sin él → 428; viejo → 412.
- Teléfono y contraseña nunca en logs.
- El trabajo nocturno siempre con candado; sin candado no corre.
- Sin datos de ciudades en el código.

3. Pruebas
- Las de la spec §Tests, con MongoDB y Redis reales y dos instancias de la app para el candado. Las del service (`Integration/Usuarios service`) viven en `tests/integration`; `DELETE /yo/historial` da 503 si falla un módulo y una prueba comprueba que reintentar termina el borrado.

4. Secuencia
1. Validators nuevos y `usuarios` completo (índices).
2. Registro y correo disponible (usa sesión de `nv-04`).
3. Perfil: `yo`, `PATCH`, correo.
4. Gustos, preferencias, privacidad, teléfono, modo.
5. Ciudades e importación.
6. ARCO: `Purger`/`Exporter`, exportar, historial, eliminar, cancelar.
7. Trabajo nocturno con candado.
8. Docs, gates y `Memoria.md`.

5. Verificación
- `go build ./... && go vet ./...`
- `go test ./internal/... ./tests/... -count=1 → verdes`
- `scripts/critic-guardrails.sh → PASS` · `scripts/check_endpoints.sh → OK` · `scripts/check_sdd.sh → SDD-OK`
- `scripts/check_coverage.sh --min 45 → OK` · `semgrep scan --config=auto --config=.semgrep/semgrep.yaml --error → 0`
