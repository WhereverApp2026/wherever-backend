# gates.md — NV-89 Reservar todo el plan y permisos al compartir

Evidencia local del 2026-11-23, en Docker `golang:1.27.3` (suites con contenedor con `--network host` y `-run`). Las suites completas las corre el CI del PR.

| Gate | Criterio | Evidencia | Estado |
|---|---|---|---|
| critic B1–B4 | Sin secretos; rutas en `docs/endpoints.md` | `critic-guardrails: PASS`; `check_endpoints: OK (285 rutas)` | ✅ |
| Build, vet y lint | Sin errores; 0 hallazgos | `go build ./...` y `go vet ./...` sin errores; golangci-lint v2.15.0 `0 issues` | ✅ |
| Unit | Reglas puras de la spec | `internal/reservas/plan_test.go` (turno que contiene la llegada, misma parada solo con reservas que ocupan, estado por reserva, traducción de rechazos, error del servicio corta, DTO), `internal/grupos/permiso_test.go` (permiso efectivo, enlace `ver` por omisión, DTOs, `permiso_plan` en la vista), `TestRateLimit_TakeCuentaVariasDeUnaVez` | ✅ |
| Integration | Casos de la sección Tests | `tests/integration/planes_reservar_test.go`: éxito parcial (reservada, sin cupo, no acepta), misma llave = misma reserva, otra llave = `ya_reservada`; sin llave 400, personas 21 y plan pasado 422, ajeno 404; límite por parada 429 sin reservar; 6 en paralelo = 1 reserva; permisos por enlace y por organizador; edición con `If-Match` (409 con versión vieja); integrante con `editar` reserva y con `ver` 403. Además `Grupos`, `Planes`, `Reservas`, `Votaciones`, `Chat` y `PresupuestoRedis` en local: ok (212 s) | ✅ local, CI pendiente |
| Security | 401/403, propiedad, privacidad y límites | `tests/security/planes_permisos_security_test.go` (sin token 401, cuenta pendiente 403, plan ajeno 404 sin crear nada ni revelar al dueño, Redis caído 503 sin guardar) + `TestPermisosPlan_*` (integrante con `ver` 403, no cambia permisos 403, no borra ni comparte 404, visibilidad 403) + `PlanesPermisos`, `Grupos`, `Reservas` y `Planes` de Security en local | ✅ local, CI pendiente |
| Database | Índices nuevos (si los hay) | Ninguno nuevo (la lectura de reservas del plan usa `ix_agenda_negocio`) | ✅ n/a |
| Contract | Formas nuevas capturadas | `planes/reservar_todo_200`, `planes/compartir_201`, `planes/detalle_integrante_200` (`mi_permiso`), `grupos/permiso_integrante_200`; `grupos/punto_encuentro_200` agrega `permiso_plan`. Suite completa de Contract en local: ok | ✅ local, CI pendiente |
| Redis | Costo medido | `TestPresupuestoRedis_ReservarTodoNV89`: 2 paradas = escritura + ventana una vez + 6 por parada (22 comandos); repetida = escritura + 2 por parada (11); con Redis caído, 503 | ✅ |
| SDD | `check_sdd` OK y `Memoria.md` al día | `SDD-OK (91 specs)`; `Memoria.md` con la entrada de NV-89 | ✅ |
