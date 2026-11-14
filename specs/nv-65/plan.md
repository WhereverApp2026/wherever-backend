# plan.md — NV-65 Eventos, ferias y tianguis

1. Mapeo
- Crear: `internal/eventos/{model,repository,service,handler,dto,privacy}.go`, `internal/testutil/eventostest` y pruebas de las cinco suites.
- Modificar: `internal/indexes` (índices de `eventos` en `scripts/indexes` y `/ready`), `cmd/api` (rutas y registro ARCO), `scripts/routes`, `docs/endpoints.md` y `docs/rate-limit-matrix.md` (`eventos_publicar`).
- Usa sin cambiar: `negocios` (dueño y estado), `moderacion` (texto de nv-42), `imagenes` (URL del bucket), `shared/pagination`.

2. Guardarraíles
- Servicios reales en las pruebas, rol y propiedad validados en el service, `If-Match` en PATCH y DELETE, cada ruta nueva en `docs/endpoints.md`.
- La política nueva se mide con `TestPresupuestoRedis_*`.

3. Secuencia
1. Preparación: colección, índices y validators.
2. Implementación: lista y detalle; crear, editar y borrar con permisos; ARCO.
3. Cierre: pruebas, documentos y `Memoria.md`.

4. Verificación
- `go build`, `go vet`, cinco suites, `critic`, `check_endpoints`, `check_sdd`, cobertura ≥ 45 %, Semgrep en 0.
