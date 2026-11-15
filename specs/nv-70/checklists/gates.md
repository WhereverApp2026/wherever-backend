# gates.md — NV-70 Perfil de persona, silenciar amigos y amigos en contactos

| Gate | Criterio | Evidencia | Estado |
|---|---|---|---|
| critic B1 | Sin secretos nuevos en Git | `critic-guardrails: PASS` (2026-11-15) | ✅ |
| critic B2 | Sin teléfonos, hashes ni tokens en logs ni respuestas | `TestPersonasPrivacidad_NadaSensibleSale`, `TestPersonasContactos_EncuentraSinGuardarNada` | ✅ |
| critic B3 | Cambios de infraestructura o pipeline aprobados | sin cambios | ✅ (no aplica) |
| critic B4 | Endpoints coinciden con `docs/endpoints.md` | `check_endpoints: OK (235 rutas)` | ✅ |
| Build, vet y lint | Sin errores; 0 hallazgos | `golangci-lint` v2.15.0: 0 issues | ✅ |
| Unit | Verde | `perfil_test.go` (forma según relación, dedupe con `indice`) | ✅ |
| Integration | perfil, silencio, contactos y presupuesto de Redis | local en verde (con `TestPersonas_*`, `TestAmistades_*`, `TestSugerencias*`) | ✅ local, CI pendiente |
| Security | `personas_privacidad_test.go` | local en verde | ✅ local, CI pendiente |
| Contract | `personas_perfil_contract_test.go` | capturado y validado en local | ✅ local, CI pendiente |
| Database | sin índices nuevos | reusa `ux_telefono_hash` y `ux_par` | ✅ |
| Cobertura y Semgrep | ≥ 45 % y 0 hallazgos | lo corre el CI del PR | pendiente (CI) |
| SDD | `check_sdd` OK y `Memoria.md` actualizado | `SDD-OK`; sección NV-70 en `Memoria.md` | ✅ |
| Criterio | Un no amigo no ve más que la búsqueda; bloqueo = inexistente; la agenda no se guarda | `TestPersonasPerfil_BasicoCompletoY404`, `countAll` antes y después | ✅ |
