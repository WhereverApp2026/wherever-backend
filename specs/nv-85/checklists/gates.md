# gates.md — NV-85 Filtros y tarjetas de Explorar

Evidencia local del 2026-11-22, en Docker `golang:1.27.3`. Las suites completas las corre el CI del PR.

| Gate | Criterio | Evidencia | Estado |
|---|---|---|---|
| critic B1–B4 | Sin secretos; rutas en `docs/endpoints.md` | `critic-guardrails: PASS`; `check_endpoints: OK (274 rutas)` | ✅ |
| Build, vet y lint | Sin errores; 0 hallazgos | `go build ./...` y `go vet ./...` sin errores; golangci-lint v2.15.0 `0 issues` | ✅ |
| Unit | Reglas puras de la spec | `go test ./internal/...` verde; `internal/negocios/explorar_test.go` (parser con filtros embebidos, categorías y precio, 401 sin sesión, cursor atado a quien consulta, afluencia, mejor cupón, iniciales) | ✅ |
| Integration | Casos de la sección Tests | `tests/integration/explorar_test.go` (filtros y combinaciones con lista = conteo = mapa; tarjeta con Sello, afluencia y mejor cupón; «De amigos» sin quien oculta su actividad, no amigos ni check-ins viejos o no válidos) + `TestLugares*`, conteo y `TestPresupuestoRedis_VistasNV66` en local con `-run`: ok | ✅ local, CI pendiente |
| Security | 401/403, propiedad, privacidad y límites | `tests/security/explorar_security_test.go` (`de_amigos` 401/403 en las 3 rutas, inyección en `categorias`/`precio` = 422, solo iniciales de amigos que comparten): ok | ✅ local, CI pendiente |
| Database | Índices nuevos (si los hay) | `ix_negocios_sello` en `tests/database/indexes_test.go`: ok. **Crearlo en Atlas antes del despliegue.** | ✅ local, CI pendiente |
| Contract | Formas nuevas capturadas | `negocios/lugares_tarjeta_200` nueva y `negocios/lugares_200` con `sello_excelencia`, `afluencia_ahora`, `cupon`: ok | ✅ local, CI pendiente |
| SDD | `check_sdd` OK y `Memoria.md` al día | `SDD-OK (91 specs)`; `Memoria.md` con la entrada de NV-85 | ✅ |
