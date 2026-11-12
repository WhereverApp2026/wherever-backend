# plan.md — NV-25 Clima e Inicio

1. Mapeo
- Crear: `internal/clima` (`clima.go`, `openmeteo.go`, `servicio.go`, `ubicacion.go`, `handler.go`), `internal/inicio` (`inicio.go`, `handler.go`), `internal/testutil/climatest`, `tests/integration/inicio_test.go`.
- Modificar: `internal/recomendador` (`Contexto.ClimaAdverso`, `Puntaje.Clima`), `internal/recomendaciones/service.go` (`ForHome`), `cmd/api/main.go`, `internal/testutil/authtest`, `scripts/routes/main.go`, `docs/endpoints.md`.
- Sin cambios: colecciones, índices, ARCO, Redis, configuración y `.env.example` (la URL de Open-Meteo es pública y fija en `clima.OpenMeteoURL`).

2. Guardarraíles
- Open-Meteo es la única interfaz simulada (`climatest`); bases reales en las pruebas, sin secretos, sin coordenadas en logs, cada ruta nueva en `docs/endpoints.md` y `scripts/routes`.
- Los filtros duros se aplican en la consulta, no después de puntuar (sin cambios de nv-23).

3. Secuencia
1. Factor clima en el recomendador.
2. Paquete `clima` y su ruta.
3. `ForHome` e Inicio; cableado.
4. Pruebas, documentos y `Memoria.md`.

4. Verificación
- Local (`AGENTS.md` §4): `go build`, `go vet`, Unit, `golangci-lint`, `critic`, `check_endpoints`, `check_sdd`. Integration, Security, Database y Contract en el CI del PR.
