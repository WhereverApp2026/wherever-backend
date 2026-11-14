# gates.md — NV-63 Postales, recuerdos y momentos

| Gate | Criterio | Evidencia | Estado |
|---|---|---|---|
| critic B1 | Sin secretos nuevos en Git ni Terraform | `scripts/critic-guardrails.sh` PASS (2026-11-14) | ✅ |
| critic B2 | Sin textos, URLs firmadas ni datos personales en logs | Logs solo con ids, estilo, visibilidad y estado; `TestPostales_CrearConImagenPropiaAjenaOUsada` revisa que el texto y la URL no salgan | ✅ |
| critic B3 | Cambios de infraestructura o pipeline aprobados por el revisor (si aplica) | No aplica: sin Terraform, pipeline ni variables nuevas | ✅ |
| critic B4 | Endpoints coinciden con `docs/endpoints.md` | `scripts/check_endpoints.sh` OK (17 filas nv-63) | ✅ |
| Build y lint | `go build`, `go vet`, Unit, `golangci-lint` | Local en Docker `golang:1.27.3`: build, vet y Unit verdes; `golangci-lint` v2.15.0 con 0 hallazgos | ✅ |
| Pruebas | Integration, Security, Database y Contract con servicios reales | Local con contenedores: `TestPostales_*`, `TestRecuerdos_*`, `TestMomentos_*`, `TestPostalesSecurity_*`, `TestIndexes_*`, `TestContract_Usuarios` y `TestImagenes_ColaDeRevision*` verdes; el resto en el CI del PR | ✅ local / CI pendiente |
| Cobertura | Mínimo 45 % | `check_coverage` en el CI del PR | pendiente (CI) |
| Semgrep | 0 hallazgos | job `semgrep` del PR | pendiente (CI) |
| SDD | `check_sdd` OK y `Memoria.md` actualizado | `scripts/check_sdd.sh` SDD-OK | ✅ |
| Visibilidad | Privada, amigos, pública y bloqueos; nada visible sin moderación aprobada | `tests/integration/postales_test.go` (`TestPostales_VisibilidadYBloqueos`), `momentos_test.go` (`TestMomentos_PublicarFeedYEtiquetas`), Unit `TestPuedeVer_Matriz` | ✅ |
| Privacidad | Sin EXIF ni GPS; ARCO borra y exporta solo lo propio | nv-45 recodifica; `TestPostales_ARCO` (exporta lo propio y solo ids de momentos ajenos; borra y quita de etiquetados) | ✅ |
