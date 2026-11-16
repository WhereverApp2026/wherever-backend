# plan.md — NV-74 Estadísticas y reporte mensual del negocio

1. Mapeo
- Crear: `internal/estadisticas/{model,repository,trabajo,service,handler,dto,fuentes}.go`, `internal/estadisticas/pdf` (escritor mínimo), `internal/testutil/estadisticastest` y pruebas de las cinco suites.
- Modificar: `internal/vistas` (interfaz `Contador` y `$inc` al escribir una vista), `internal/indexes` + `scripts/indexes` (`ux_negocio_fecha`, `ux_categoria_mes`), `cmd/api` (fuentes, trabajo y observador de baja), `scripts/routes`, `docs/endpoints.md`.
- Sin cambios: panel de nv-56, variables de entorno, políticas de Redis, `go.mod` (salvo que el revisor elija `gopdf`).

2. Guardarraíles
- Servicios reales en las pruebas; ninguna colección de estadísticas con `usuario_id`; categoría con ≥ 5 negocios; sin Redis en el trabajo ni en el contador.
- `TestPresupuestoRedis_EstadisticasNV74` fija el costo.

3. Secuencia
1. Preparación: colecciones e índices; fuentes por interfaz.
2. Implementación: contador de vistas; trabajo por hora y nocturno con relleno de 90 días; `GET /estadisticas`; PDF y `GET /reportes/mensual`.
3. Cierre: pruebas, documentos y `Memoria.md`.

4. Verificación
- `go build`, `go vet`, Unit y lint en local; Integration, Security, Database y Contract en el CI del PR; `critic`, `check_endpoints`, `check_sdd`, cobertura ≥ 45 %, Semgrep en 0.
