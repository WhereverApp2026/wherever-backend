# gates.md — NV-88 Cupo por hora y día del negocio

Evidencia local del 2026-11-23, en Docker `golang:1.27.3`. Las suites completas las corre el CI del PR.

| Gate | Criterio | Evidencia | Estado |
|---|---|---|---|
| critic B1–B4 | Sin secretos; rutas en `docs/endpoints.md` | `critic-guardrails: PASS`; `check_endpoints: OK (283 rutas)` | ✅ |
| Build, vet y lint | Sin errores; 0 hallazgos | `go build ./...` y `go vet ./...` sin errores; golangci-lint v2.15.0 `0 issues` | ✅ |
| Unit | Reglas puras de la spec | `go test ./internal/... ./cmd/...` verde; `internal/reservas/disponibilidad_test.go` (cupo con entrada, por omisión, 0 = cerrado, turno en dos horas, madrugada en el día siguiente; afluencia por turno; sugerencia a ±2 h, empate y sin curva) e `internal/negocios/panel/cupo_test.go` (horas dentro del horario, repetidas, HH:00, tope de `max_personas`, validador, tabla de 7 días con afluencia, reservas-config conserva el cupo) | ✅ |
| Integration | Casos de la sección Tests | `tests/integration/reservas_cupo_test.go` (cupos distintos por día; `If-Match` y 409; fuera de horario y repetida 422; sin `If-Match` 400; ninguna reserva supera el cupo; afluencia y sugerencia; `hora` mal formada 422; `mesas` y `max_personas`) + `TestReservas_*` y `TestNegociosPanel*` en local con `-run`: ok | ✅ local, CI pendiente |
| Security | 401/403, propiedad, privacidad y límites | `tests/security/reservas_cupo_security_test.go` (GET y PUT: sin token 401, consumidor 403, dueño ajeno 404 y la tabla no cambia) + `TestNegociosPanelSecurity_*` y `TestReservas*` | ok en local con `-run`; ✅ local, CI pendiente |
| Database | Índices nuevos (si los hay) | Ninguno nuevo | ✅ n/a |
| Contract | Formas nuevas capturadas | `reservas/disponibilidad_200` (con `afluencia` y `sugerida`), `negocios/cupo_200`, `negocios/cupo_put_200` y `negocios/reservas_config_200` con `mesas` y `max_personas` | capturadas con `CONTRACT_CAPTURE=1` y `-run`; las capturas que ya existían en esas pruebas no cambiaron; ✅ local, CI pendiente |
| Redis | Sin costo nuevo | Mismo candado `lock:reserva:{negocio}:{hora}` e idempotencia (503 con Redis caído); la disponibilidad y el cupo solo leen MongoDB; `TestPresupuestoRedis_*` | `TestPresupuestoRedis_*` ok en local; ✅ |
| SDD | `check_sdd` OK y `Memoria.md` al día | `SDD-OK (91 specs)`; `Memoria.md` con la entrada de NV-88 | ✅ |
