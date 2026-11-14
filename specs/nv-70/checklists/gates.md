# gates.md — NV-70 Perfil de persona, silenciar amigos y amigos en contactos

Spec en Borrador (2026-11-13), sin ejecutar. Se llena al implementar; Integration, Security, Database y Contract las corre el CI del PR (`AGENTS.md` §4). Sin firma.

| Gate | Criterio | Evidencia | Estado |
|---|---|---|---|
| critic B1 | Sin secretos nuevos en Git | — | pendiente |
| critic B2 | Sin teléfonos, hashes de teléfono ni tokens en logs o colecciones | — | pendiente |
| critic B3 | Cambios de infraestructura o pipeline aprobados | No aplica | pendiente |
| critic B4 | Endpoints coinciden con `docs/endpoints.md` | — | pendiente |
| Build y vet | Sin errores | — | pendiente |
| Lint | 0 hallazgos | — | pendiente |
| Unit | Verde | — | pendiente |
| Integration | `personas_perfil_test.go`, `amistades_silencio_test.go`, `personas_contactos_test.go`, `TestPresupuestoRedis_VentanasPorUsuarioNV70` | — | pendiente |
| Security | `personas_privacidad_test.go` | — | pendiente |
| Database y Contract | sin índices nuevos; `personas_perfil_contract_test.go` | — | pendiente |
| Cobertura y Semgrep | ≥ 45 % y 0 hallazgos | — | pendiente |
| SDD | `check_sdd` OK y `Memoria.md` actualizado | — | pendiente |
