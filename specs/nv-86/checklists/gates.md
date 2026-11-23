# gates.md — NV-86 Notificaciones por categoría y avisos nuevos

Evidencia local del 2026-11-23, en Docker `golang:1.27.3`. Las suites completas las corre el CI del PR.

| Gate | Criterio | Evidencia | Estado |
|---|---|---|---|
| critic B1–B4 | Sin secretos; rutas en `docs/endpoints.md` | `critic-guardrails: PASS`; `check_endpoints: OK (274 rutas)` | ✅ |
| Build, vet y lint | Sin errores; 0 hallazgos | `go build ./...` y `go vet ./...` sin errores; golangci-lint v2.15.0 `0 issues` | ✅ |
| Unit | Reglas puras de la spec | `go test ./internal/... ./cmd/...` verde; tipo → categoría, conteo por categoría, interruptores y horario silencioso (`categoria_test.go`), hora de salir (`salida_test.go`), horario del cupón (`avisos_test.go`) | ✅ |
| Integration | Casos de la sección Tests | `tests/integration/notificaciones_categorias_test.go`: filtro y conteos, cursor atado a la pestaña, `social` apagado no guarda, cada aviso nuevo una sola vez, `cupones`/`recordatorios_plan` apagados y horario silencioso | ✅ local (`-run "TestNotificaciones_|TestReacciones|TestUsuarios"`), CI pendiente |
| Security | 401/403, propiedad, privacidad y límites | `TestNotificacionesSecurity_CategoriasSoloMiasYSoloIDs`: 401 sin sesión, la pestaña y los conteos son solo míos, `datos` solo ids | ✅ local, CI pendiente |
| Database | Índices nuevos (si los hay) | `ix_notif_categoria` e `ix_checkins_pendientes` en `tests/database/indexes_test.go`. **Crearlos en Atlas antes del despliegue.** | ✅ local (`TestIndexes`), CI pendiente |
| Contract | Formas nuevas capturadas | `notificaciones/lista_social_200`, `notificaciones/no_leidas_200` y `notificaciones.social`/`recordatorios_plan` en las 5 capturas de `usuarios/*` | ✅ local (`TestContract_NotificacionesCategorias`, `TestContract_Usuarios`), CI pendiente |
| SDD | `check_sdd` OK y `Memoria.md` al día | `SDD-OK (91 specs)`; `Memoria.md` con la entrada de NV-86 | ✅ |
