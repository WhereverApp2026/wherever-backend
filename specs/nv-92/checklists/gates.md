# gates.md — NV-92 Corazón de favorito en las tarjetas

Evidencia local del 2026-11-25, en Docker `golang:1.27.3`. Las suites completas las corre el CI del PR.

| Gate | Criterio | Evidencia | Estado |
|---|---|---|---|
| Build, vet y lint | Sin errores; 0 hallazgos | `go build ./...` y `go vet ./...` sin salida; golangci-lint v2.15.0: `0 issues.` | ✅ |
| Unit | Armado de `es_favorito` | `go test ./internal/... ./cmd/...` verde: `internal/negocios/favoritos_test.go` | ✅ |
| Integration | Uno guardado y otro no; sin sesión se omite; otra persona ve `false` | `-run 'TestFavoritosTarjetas\|TestFavoritos_\|TestInicio_\|TestRecomendaciones_\|TestLugares_OptionalAuth'` ok (58,9 s) | ✅ |
| Security | Sin rutas ni autorización nuevas | Cubierto por las suites de nv-21, nv-24, nv-25 y nv-54 (CI) | ✅ N/A |
| Database | Índices nuevos | Sin índices nuevos (usa el prefijo de `ux_favorito`) | ✅ N/A |
| Contract | Tarjeta de Explorar | `CONTRACT_CAPTURE=1 -run TestContract_ExplorarTarjeta` ok (23,9 s): `negocios/lugares_tarjeta_200` suma `es_favorito: bool` | ✅ |
| SDD | `check_sdd` OK y `Memoria.md` al día | `SDD-OK (93 specs)`; `check_endpoints: OK (285 rutas)`; `critic-guardrails: PASS` | ✅ |
