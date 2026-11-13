# plan.md — NV-62 Check-in y reseñas

1. Mapeo
- Crear: `internal/resenas` (check-ins, reseñas, útiles, recálculo, `Privacy`), `internal/testutil/resenastest`, pruebas de las cinco suites.
- Modificar: `internal/negocios` (subdocumento interno `calificacion_suma` y método atómico de recálculo; `calificacion` y `qr_checkin` ya existen desde nv-20), `internal/indexes.All()`, rutas en `internal/http`, gancho de publicación en la cola de nv-43, registro de `Purger`/`Exporter`, `docs/endpoints.md` y `docs/rate-limit-matrix.md` (al programar).
- Sin cambios: `negocios.Aspectos` (5 fijos), `ux_qr_checkin`.

2. Guardarraíles
- Servicios reales en las pruebas; solo la moderación (nv-42) y la IA se simulan como interfaces externas.
- Sin coordenadas del usuario, tokens ni datos personales en logs ni en la base.
- Recálculo en un solo update con pipeline; la bandera `contada` evita contar dos veces.
- Cada llave nueva de Redis con TTL; sin `MULTI`/`EXEC`.

3. Secuencia
1. Preparación: filas de J7 a `docs/endpoints.md`; índices.
2. Implementación: check-in → por reseñar → crear reseña con moderación → recálculo → editar y borrar → útil → respuesta e IA → reportes → lista pública y resumen → ARCO.
3. Cierre: pruebas, gates y `Memoria.md`.

4. Verificación
- `go build`, `go vet`, Unit y lint en local; Integration, Security, Database y Contract en el CI del PR; `critic`, `check_endpoints`, `check_sdd`, cobertura ≥ 45 %, Semgrep en 0.
