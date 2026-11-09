# plan.md — NV-40 Dispositivos y centro de notificaciones

1. Mapeo
- Crear: `internal/notificaciones/*`, `internal/dispositivos/*`, `internal/platform/fcm/*`, pruebas en `tests/`.
- Modificar: `internal/amistades`, `internal/grupos` (emitir eventos), `internal/platform/config`, `.env.example`, `docs/endpoints.md`, `Memoria.md`.
- Sin cambios: lo que esta spec no nombra.

2. Guardarraíles
- Servicios reales en las pruebas, sin secretos en Git, sin datos personales en logs, roles validados en cada endpoint y cada ruta nueva en `docs/endpoints.md`.
- La cuenta de servicio nunca entra a Git.

3. Secuencia
1. Preparación
2. Implementación
3. Cierre

4. Verificación
- `go build`, `go vet`, cinco suites, `critic`, `check_endpoints`, `check_sdd`, cobertura ≥ 45 %, Semgrep en 0.
