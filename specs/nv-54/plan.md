# plan.md — NV-54 Favoritos de negocios y planes

1. Mapeo
- Crear: `internal/favoritos/{model,repository,service,handler,dto,privacy}.go`, pruebas de las cuatro suites (el módulo se arma en `internal/testutil/authtest`, sin paquete `favoritostest`).
- Modificar: `internal/indexes` (lo usan `scripts/indexes` y `/ready`); lista de módulos y registro de `Privacy` en `cmd/api`, `authtest` y `scripts/routes`; `docs/endpoints.md` (filas de A3); contrato de la exportación ARCO.
- `negocios`: `Cover` exportada y `FindFavoriteCards` (solo lectura). Sin cambios: planes (se leen por interfaz), Redis y variables.

2. Guardarraíles
- Servicios reales en las pruebas, sin secretos en Git, sin datos personales en logs, rol validado en cada endpoint y cada ruta nueva en `docs/endpoints.md`.
- El objetivo se valida en el service: nunca se guarda un plan ajeno ni un negocio no aprobado.

3. Secuencia
1. Preparación: índices y modelo.
2. Implementación: marcar y quitar, lista con negocios, ids y búsqueda, ARCO.
3. Cierre: pruebas, documentos y `Memoria.md`.

4. Verificación
- `go build`, `go vet`, cinco suites, `critic`, `check_endpoints`, `check_sdd`, cobertura ≥ 45 %, Semgrep en 0.
