# gates.md — NV-65 Eventos, ferias y tianguis

| Gate | Criterio | Evidencia | Estado |
|---|---|---|---|
| critic B1 | Sin secretos nuevos en Git | `critic-guardrails: PASS` (2026-11-15) | ✅ |
| critic B2 | Sin tokens ni datos personales en logs | el aviso de cola fallida solo lleva el id del evento | ✅ |
| critic B3 | Cambios de infraestructura o pipeline aprobados | sin cambios; índices nuevos por `scripts/indexes` | ✅ (no aplica) |
| critic B4 | Endpoints coinciden con `docs/endpoints.md` | `check_endpoints: OK (237 rutas)` | ✅ |
| Build, vet y lint | Sin errores; 0 hallazgos | `golangci-lint` v2.15.0: 0 issues | ✅ |
| Unit | Verde | `service_test.go` (fechas, foto, permisos, negocio publicable, estados, DTO) | ✅ |
| Integration | `eventos_test.go` | local en verde | ✅ local, CI pendiente |
| Security | `eventos_security_test.go` | local en verde | ✅ local, CI pendiente |
| Database | los cuatro índices | `tests/database` local en verde | ✅ local, CI pendiente |
| Contract | `eventos_contract_test.go` | capturado y validado en local | ✅ local, CI pendiente |
| Cobertura y Semgrep | ≥ 45 % y 0 hallazgos | lo corre el CI del PR | pendiente (CI) |
| SDD | `check_sdd` OK y `Memoria.md` actualizado | `SDD-OK`; sección NV-65 en `Memoria.md` | ✅ |
| Criterio | Consumidor no publica; comerciante solo lo suyo; nada terminado, borrador o cancelado en la lista | `TestEventos_*`, `TestEventosSecurity_RolesYPropiedad` | ✅ |
