# plan.md — NV-63 Postales, recuerdos y momentos

1. Mapeo
- Crear: `internal/postales/*` (postales y recuerdos), `internal/momentos/*` (momentos, feed y panel del negocio), cada uno con su `Privacy`; `internal/testutil/postalestest` y pruebas en `tests/`.
- Modificar:
  - `internal/imagenes`: consultar el estado y el dueño de una imagen.
  - `internal/notificaciones`: tres tipos nuevos.
  - `internal/negocios`: propietario para el panel.
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
