# plan.md — NV-57 Chat de grupo

1. Mapeo
- Crear: `internal/chat/*` (modelo, repositorio, service, handler, DTO y `Privacy`; `tiemporeal.go` si se aprueba el canal), `internal/testutil/chattest` y pruebas en `tests/`.
- Modificar:
  - `internal/grupos`: `ultimo_mensaje` y consulta de integrante confirmado.
  - `internal/votaciones`: aviso con `PublicarSistema`.
  - `internal/notificaciones`: tipo `mensaje_grupo`.
  - `scripts/indexes`, `cmd/api`, `docs/endpoints.md`, `docs/rate-limit-matrix.md` y `Memoria.md`.
- Sin cambios: lo que esta spec no nombra.

2. Guardarraíles
- Servicios reales en las pruebas y sin secretos en Git.
- Sin texto de mensajes ni tokens en logs.
- Integrante confirmado validado en cada ruta y al abrir el canal.
- Cada ruta nueva en `docs/endpoints.md`.
- Medir los comandos de Redis antes de fijar el límite y la opción de tiempo real.

3. Secuencia
1. Preparación: aprobar la spec y la opción de tiempo real; nv-37 fusionada.
2. Implementación: REST, ARCO y push; después, el canal.
3. Cierre.

4. Verificación
- `go build`, `go vet`, cinco suites, `critic`, `check_endpoints`, `check_sdd`, cobertura ≥ 45 %, Semgrep en 0.
