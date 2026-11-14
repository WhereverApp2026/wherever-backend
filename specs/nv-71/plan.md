# plan.md — NV-71 Dispositivos por id y reglas de moderación por filtro

1. Mapeo
- Crear: pruebas de las suites (`dispositivos_id`, `moderacion_reglas_filtro`, `dispositivos_reglas_security`, contrato).
- Modificar: `internal/notificaciones` (rutas `/:id`, `Location` al registrar, `app_version` y `canales_activos` en `Device`, filtro por canal al enviar, exportación ARCO), `internal/moderacion` (mapa `filtros` en las reglas, rutas `/reglas/:id`, decisión del `Moderador` por filtro), `internal/shared/validation` (`canal`), `scripts/routes` y `docs/endpoints.md`.
- Sin cambios: índices (no hay colección nueva), variables, rutas de nv-40 y nv-43 (salvo la cabecera `Location`).

2. Guardarraíles
- Servicios reales en las pruebas, sin secretos en Git, el token de FCM nunca en rutas, respuestas ni logs, rol y dueño validados en el service, y cada ruta nueva en `docs/endpoints.md`.
- Las reglas se cambian siempre con `If-Match`.

3. Secuencia
1. Preparación: decisiones del revisor y filas en `docs/endpoints.md`.
2. Implementación: dispositivos por id y canales; reglas por filtro y `Moderador`.
3. Cierre: pruebas, documentos y `Memoria.md`.

4. Verificación
- `go build`, `go vet`, cinco suites, `critic`, `check_endpoints`, `check_sdd`, cobertura ≥ 45 %, Semgrep en 0.
