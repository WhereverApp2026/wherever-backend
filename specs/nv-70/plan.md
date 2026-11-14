# plan.md — NV-70 Perfil de persona, silenciar amigos y amigos en contactos

1. Mapeo
- Crear: pruebas de las cinco suites (`personas_perfil`, `amistades_silencio`, `personas_contactos`, `personas_privacidad`, contrato).
- Modificar: `internal/personas` (handler, service y DTO de perfil y contactos), `internal/amistades` (`silenciado_por`, ruta de silencio, `silenciado` en la lista y en la exportación), `internal/usuarios` (`FindByPhoneHashes` con `$in`), `internal/grupos` (conteo de grupos en común), `internal/grafo` (amigos en común de un par, reutiliza nv-53), `cmd/api` y `scripts/routes`, `docs/endpoints.md` y `docs/rate-limit-matrix.md`.
- Sin cambios: índices (`ux_telefono_hash` y `ux_par` ya existen), variables (`PHONE_HMAC_KEY` ya existe).

2. Guardarraíles
- Servicios reales en las pruebas (Neo4j caído con el stack real), sin secretos en Git, sin números ni hashes de teléfono en logs ni en ninguna colección, rol y relación validados en el service, y cada ruta nueva en `docs/endpoints.md`.
- Cada política nueva de Redis se mide con `TestPresupuestoRedis_*`.

3. Secuencia
1. Preparación: decisiones del revisor (teléfonos en claro, límites, 404 por bloqueo) y filas en `docs/endpoints.md`.
2. Implementación: silencio en `amistades`; perfil de persona; contactos.
3. Cierre: pruebas, documentos y `Memoria.md`.

4. Verificación
- `go build`, `go vet`, cinco suites, `critic`, `check_endpoints`, `check_sdd`, cobertura ≥ 45 %, Semgrep en 0.
