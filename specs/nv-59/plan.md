# plan.md — NV-59 Reservas y disponibilidad

1. Mapeo
- Crear: `internal/reservas/` (model, repository, service, handler, dto, `disponibilidad.go`, `reembolso.go`, barrido, privacy), `internal/testutil/reservastest`, `tests/integration/reservas_test.go`, `tests/security/reservas_security_test.go`.
- Modificar: `scripts/indexes` (`reservas`, `bloqueos_horario`), `internal/http` (rutas), `internal/privacidad` (registro), `internal/negocios` (lectura de `reservas_config` si nv-55 no la expone), `docs/endpoints.md` (12 filas), `docs/rate-limit-matrix.md` (política `reservas`), `tests/database/indexes_test.go`, fixtures de contrato.
- Sin cambios: el cobro (nv-60); `PUT /negocios/{id}/reservas-config` (nv-55).

2. Guardarraíles
- Servicios reales en las pruebas, sin secretos en Git, sin datos personales en logs, roles validados en cada endpoint y cada ruta nueva en `docs/endpoints.md`.
- Candado `lock:reserva:{negocio}:{fecha_hora}` e `idem:reserva:{usuario}:{llave}` con TTL; Redis caído = 503.
- La agenda del negocio no expone correo ni teléfono del cliente.

3. Secuencia
1. Preparación: nv-55 en `main` con `reservas_config`; decisiones resueltas.
2. Implementación: cálculo puro (disponibilidad, reembolso, código) con pruebas; repositorio e índices; crear con candado e idempotencia; resto de rutas; barrido.
3. Cierre: privacidad, contrato, documentos y `Memoria.md`.

4. Verificación
- `go build`, `go vet`, cinco suites, `critic`, `check_endpoints`, `check_sdd`, cobertura ≥ 45 %, Semgrep en 0.
