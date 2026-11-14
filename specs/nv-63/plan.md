# plan.md — NV-63 Postales, recuerdos y momentos

1. Mapeo
- Crear: `internal/postales/*` (postales, recuerdos, momentos, feed, panel del negocio y destinos de la cola) con tres `Privacy`; `cmd/api/postales.go`; pruebas en `tests/` (`postales_test.go`, `recuerdos_test.go`, `momentos_test.go`, `postales_security_test.go`). Sin `postalestest`: `authtest` arma el módulo con el doble de imágenes de nv-45 y el moderador de texto de `resenastest`.
- Modificar:
  - `internal/imagenes`: `Service.Consultar` (estado y URL pública de una imagen propia).
  - `internal/notificaciones`: tres tipos nuevos.
  - `internal/negocios`: sin cambios (el panel usa `FindByID` y el recuerdo `FindFavoriteCards`).
  - `internal/testutil/authtest`, `internal/indexes`, `scripts/routes`, `cmd/api/resenas.go` (la guardia de IA modera también estos textos).
  - `scripts/indexes`, `cmd/api`, `docs/endpoints.md`, `docs/rate-limit-matrix.md` y `Memoria.md`.
- Sin cambios: lo que esta spec no nombra.

2. Guardarraíles
- Servicios reales en las pruebas; solo S3 y Rekognition simulados por interfaz.
- Sin secretos en Git.
- Sin textos, URLs firmadas ni nombres en logs.
- Visibilidad y bloqueos validados en el service.
- Cada ruta nueva en `docs/endpoints.md`.

3. Secuencia
1. Preparación: aprobar la spec y quién dibuja; nv-42 a nv-45 fusionadas.
2. Implementación: postales, recuerdos, momentos, ARCO y avisos.
3. Cierre.

4. Verificación
- `go build`, `go vet`, cinco suites, `critic`, `check_endpoints`, `check_sdd`, cobertura ≥ 45 %, Semgrep en 0.
