# gates.md — NV-90 Datos del negocio que pide la maqueta

Evidencia local del 2026-11-23, en Docker `golang:1.27.3`. Las suites completas las corre el CI del PR.

| Gate | Criterio | Evidencia | Estado |
|---|---|---|---|
| critic B1–B4 | Sin secretos; rutas en `docs/endpoints.md` | `critic-guardrails: PASS`; `check_endpoints: OK (285 rutas)`; `gitleaks dir`: no leaks found | ✅ |
| Build, vet y lint | Sin errores; 0 hallazgos | `go build ./...` y `go vet ./...` sin salida; golangci-lint v2.15.0: `0 issues.` | ✅ |
| Unit | Reglas puras de la spec | `go test ./internal/... ./cmd/...` verde: `datos_test.go` (negocios, alta, panel), `TestPuntaje_GustosConExperiencias`, `TestContexto_AlergiasYDietaSiempreComoFiltroDuro` (asistente) | ✅ |
| Integration | Casos de la sección Tests | `-run 'TestNegociosDatos\|TestNegociosPanel_PerfilYCambiosPendientes\|TestNegociosAlta_PasoAPaso'` ok (30,9 s): alta, cambio pendiente, ficha y filtro `experiencia`. Señal del recomendador solo en Unit (ver Drift) | ✅ |
| Security | 401/403, propiedad, privacidad y límites | `-run 'TestNegociosDatosSecurity\|TestNegociosPanelSecurity'` ok (25,2 s): 401, ajeno 404, `javascript:`/`data:`/`http:`/acortador 422, ficha sin número ni datos del dueño | ✅ |
| Database | Índices nuevos (si los hay) | Sin índices nuevos | ✅ N/A |
| Contract | Formas nuevas capturadas | `negocios/lugares_ficha_datos_200` capturado con `CONTRACT_CAPTURE=1` y validado sin la variable (`TestContract_Negocios*` ok) | ✅ |
| SDD | `check_sdd` OK y `Memoria.md` al día | `SDD-OK (91 specs)`; `Memoria.md` con la sección NV-90 | ✅ |
