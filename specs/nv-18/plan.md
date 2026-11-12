# plan.md — NV-18 Caché en Redis

1. Mapeo
- Crear: `internal/platform/cache` (`cache.go`, `cache_test.go`), `tests/integration/cache_test.go`.
- Modificar: `internal/negocios` (`service.go`: `WithCache`, `Facets` e invalidación en `Import`; `lugares_service.go`: `Detail`; `repository.go`: `Upsert` con `FindOneAndUpdate`; `handler.go`: comentario de `no-store`), `cmd/api/main.go` y `internal/testutil/authtest` (armado), `tests/integration/presupuesto_redis_test.go`, `go.mod` (`golang.org/x/sync` directa).
- Sin cambios: `internal/shared/etag`, `internal/shared/pagination`, `internal/configuracion`, compresión (nv-12) y lo que esta spec no nombra.

2. Guardarraíles
- Servicios reales en las pruebas, sin secretos en Git, sin datos personales en logs (el aviso de la caché no lleva la llave), sin endpoints nuevos.
- Llaves solo `cache:{entidad}:{id}` con TTL (constitución §6).

3. Secuencia
1. Preparación: corregir el desfase de la spec.
2. Implementación: paquete `cache`, negocios, armado.
3. Cierre: pruebas, presupuesto, documentos y `Memoria.md`.

4. Verificación
- Local: `go build`, `go vet`, Unit, `golangci-lint`, `critic`, `check_endpoints`, `check_sdd`. CI: Integration, Security, Database, Contract, cobertura ≥ 45 %, Semgrep en 0.
