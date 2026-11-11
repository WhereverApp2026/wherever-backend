# plan.md — NV-31 Interpretar la frase y armar candidatos

1. Mapeo
- Crear: `internal/asistente/` (`interpretar.go`, `armar.go`, `service.go`, `dto.go`, `handler.go`, `prompts/interpretar.txt`), `internal/testutil/iatest` (doble de `ia.Modelo`), `tests/integration/asistente_test.go`, `tests/security/asistente_security_test.go`.
- Modificar: `cmd/api` (Gemini si hay llave), `internal/testutil/authtest` (módulo con el doble en `Env.IA`), `scripts/routes/main.go`, `docs/endpoints.md`, `Memoria.md`.
- Sin cambios: `internal/recomendador` (`Recomendar` ya es pública), `internal/planes` (solo se usa `Recalcular`; nv-30 trabaja ahí), `internal/indexes` (sin colección nueva), `docs/ia.md` (no existe; lo de IA está en `docs/endpoints.md` y `Memoria.md`).

2. Guardarraíles
- Servicios reales en las pruebas; solo el proveedor de IA es doble. Sin secretos en Git, sin la frase ni datos personales en logs, cuenta activa validada y la ruta en `docs/endpoints.md`.
- Al modelo solo instrucciones fijas, día y hora y la frase anonimizada.

3. Secuencia
1. Preparación: rama `nv-31` desde `origin/nv-29`; corregir drift.
2. Implementación: interpretar, candidatos, armado, endpoint, registro del módulo.
3. Cierre: pruebas, documentos, `Memoria.md`, guardrails y PR contra `nv-29`.

4. Verificación
- Local en Docker: `go build`, `go vet`, Unit y `golangci-lint` v2.15.0; `critic`, `check_endpoints`, `check_sdd`.
- CI del PR: Integration, Security, Database, Contract, cobertura ≥ 45 %, Semgrep en 0.
