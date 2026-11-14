# plan.md — NV-67 Reportes de información incorrecta de un negocio

1. Mapeo
- Crear: `internal/reportesnegocio/{model,repository,service,handler,dto,privacy}.go`, `internal/testutil/reportesnegociotest` y pruebas de las cinco suites.
- Modificar: `internal/indexes` (índices de `reportes_negocio` en `scripts/indexes` y `/ready`), `cmd/api` (rutas antes de `/lugares/:id` y registro ARCO), `scripts/routes`, `docs/endpoints.md` y `docs/rate-limit-matrix.md` (límite diario en MongoDB y rutas en la política `admin`).
- Usa sin cambiar: `negocios` (`FindVisible`, `propietario_id`), `moderacion` (texto de nv-42), `shared/pagination`.

2. Guardarraíles
- Servicios reales en las pruebas, rol validado en cada endpoint, `If-Match` en el `PATCH`, ARCO completo, cada ruta nueva en `docs/endpoints.md`.
- Sin política nueva de Redis.

3. Secuencia
1. Preparación: colección, índices y validators.
2. Implementación: crear con límites; cola y resolución del administrador; ARCO.
3. Cierre: pruebas, documentos y `Memoria.md`.

4. Verificación
- `go build`, `go vet`, cinco suites, `critic`, `check_endpoints`, `check_sdd`, cobertura ≥ 45 %, Semgrep en 0.
