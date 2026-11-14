# plan.md — NV-66 Actividad de lugares: conteo, afluencia y vistos recientemente

1. Mapeo
- Crear: `internal/vistas/{model,repository,service,handler,dto,privacy}.go`, `internal/negocios/afluencia.go`, `internal/testutil/vistastest` y pruebas de las cinco suites.
- Modificar: `internal/negocios` (conteo con `matchConditions`, `FindVisibleMany`, rutas antes de `/lugares/:id`), `internal/indexes` (índices de `historial_vistas` en `scripts/indexes` y `/ready`), `cmd/api` (orden de módulos y registro ARCO), `scripts/routes`, `docs/endpoints.md`, `docs/rate-limit-matrix.md` (límite de vistas en MongoDB) y la estimación de Redis de nv-11 en `Memoria.md`.
- Sin cambios: `busquedas`, `recomendaciones`, variables.

2. Guardarraíles
- Servicios reales en las pruebas, sin `usuario_id` con `negocio_id` en logs, ARCO completo (borrado de historial, de cuenta y exportación), cada ruta nueva en `docs/endpoints.md`.
- Sin política nueva de Redis; `TestPresupuestoRedis_VistasNV66` fija el costo.

3. Secuencia
1. Preparación: colección e índices; orden de rutas.
2. Implementación: conteo; afluencia; vistas, recientes y borrado; ARCO.
3. Cierre: pruebas, documentos y `Memoria.md`.

4. Verificación
- `go build`, `go vet`, cinco suites, `critic`, `check_endpoints`, `check_sdd`, cobertura ≥ 45 %, Semgrep en 0.
