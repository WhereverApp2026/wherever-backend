# plan.md — NV-68 Extras de planes: alternativas, cambiar parada, populares y paquete sin conexión

1. Mapeo
- Crear: `internal/planes/{alternativas,populares,paquete}.go` y sus pruebas en las cinco suites.
- Modificar: `internal/planes` (modelo con `visibilidad`, `veces_clonado`, `plan_origen_id` y `zona`; `PatchRequest`, `Clone`, `Get`, handler y rutas), `internal/planes/calculadora.go` (fija `zona`), `internal/negocios` (filtro de abiertos a una hora reutilizable desde planes), `internal/indexes` (`ix_populares`), `cmd/api`, `scripts/routes`, `docs/endpoints.md` y `docs/rate-limit-matrix.md`.
- Sin cambios: `internal/grupos` (la vista pública de NV-38 se queda como está), variables de entorno.

2. Guardarraíles
- Servicios reales en las pruebas; sin dueño ni ids de personas en respuestas públicas; punto de consulta redondeado antes de usarlo o registrarlo.
- Sin estado en memoria entre peticiones: dos instancias detrás del balanceador.
- Cada política nueva de Redis se mide con `TestPresupuestoRedis_*`.

3. Secuencia
1. Preparación: campos nuevos con valores por omisión, `zona` en `Calculadora` e índice `ix_populares`.
2. Implementación: alternativas y cambiar; visibilidad, clonar públicos y populares; paquete.
3. Cierre: pruebas, documentos y `Memoria.md`.

4. Verificación
- `go build`, `go vet`, cinco suites, `critic`, `check_endpoints`, `check_sdd`, cobertura ≥ 45 %, Semgrep en 0.
