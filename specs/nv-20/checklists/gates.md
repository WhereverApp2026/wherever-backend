# gates.md — NV-20 Negocios aprobados, categorías y carga semilla

| Gate | Criterio | Evidencia | Estado |
|---|---|---|---|
| Spec | Las 6 decisiones de `## Aprobación` firmadas por el revisor antes de codificar | spec + aprobación | pendiente de firma; el revisor pidió ejecutarla (2026-11-10) |
| critic B1 | Sin secretos nuevos en Git ni Terraform | `critic-guardrails: PASS`; la plantilla CSV solo trae ejemplos (`example.test`) | listo |
| critic B2 | Sin tokens, correos de dueños ni datos personales en logs | `critic-guardrails: PASS`; el service solo registra `creados`, `actualizados` y `errores` | listo |
| critic B3 | Sin cambios de infraestructura, imagen o pipeline | no aplica: sin cambios de infraestructura, imagen ni pipeline | listo |
| critic B4 | Las dos rutas coinciden con `docs/endpoints.md` | `check_endpoints: OK (40 rutas)` | listo |
| Modelo | `negocios` usa los nombres de A2/A5 y los estados `borrador/en_revision/aprobado/rechazado/pausado` | `internal/negocios/model.go` + Integration | listo |
| Índices | Los 9 índices existen, son idempotentes y `/ready` falla si falta uno; solo los crea `scripts/indexes` | `tests/database`: `expected` con los 9 nombres, únicos y `TestIndexes_NegociosTextAndPartial`; `/ready` usa `indexes.All()` | listo |
| Semilla formato | CSV/JSON, encabezado exacto, BOM tolerado, 1–100 filas, listas con `|`, 3+ fotos y días válidos | `internal/negocios/import_test.go` (19 pruebas) + `FuzzParseCSV` (30 s reales, 1,6 millones de ejecuciones, sin fallos) | listo |
| Semilla idempotencia | Repetir el archivo no duplica; `creados`/`actualizados` correctos; `qr_checkin` estable; `version` sube | `TestNegociosImport_SeedsAndIsIdempotent` | listo |
| Semilla parcial | Un error de fila no revierte las válidas; cada error trae `{fila, campo, mensaje}` | `TestNegociosImport_PartialErrorsAndOwnerResolution` | listo |
| Dueño | `propietario_correo` activo o admin provisional; el `sub` es ObjectId; sin correos en logs | `TestNegociosImport_PartialErrorsAndOwnerResolution` + `TestNegociosSecurity_RequiresAdminAndOnlyApprovedInCatalog` (`sub` no ObjectId = 401) | listo |
| Solo aprobados | `borrador/en_revision/rechazado/pausado` y eliminados no cuentan ni salen | `TestNegociosCategories_CountsAndExclusion` + Security | listo |
| Categorías | 6 giros en orden, etiquetas/servicios con conteo, filtros estáticos y tope 50 | `TestNegociosCategories_*` + `Contract/Negocios` | listo |
| Sin caché | `GET /categorias` no devuelve `ETag` ni 304 y lleva `no-store` | `TestNegociosCategories_CountsAndExclusion` (`no-store`, sin `ETag`, `If-None-Match` da 200) | listo |
| Admin | Sin token 401, consumidor 403, admin 200; límite `admin 200/min` antes de `RequireRoles` | `TestNegociosSecurity_RequiresAdminAndOnlyApprovedInCatalog` | listo |
| Documentos | `endpoints.md`, `rate-limit-matrix.md`, `openapi.json` y `Memoria.md` actualizados | `docs/endpoints.md`, `docs/rate-limit-matrix.md`, `openapi.json`, `Memoria.md` | listo |
| Pruebas | Unit, Integration, Security, Database y Contract verdes con `-count=1` y servicios reales | `golang:1.27.3`, 2026-11-10: Unit, Integration (207 s), Security (73 s), Database (48 s) y Contract (36 s) verdes; golangci-lint 0, gosec 0, govulncheck sin llamadas vulnerables | listo |
| Cobertura | Mínimo 45 % | `check_coverage: OK (84.1% >= 45%)` con Unit, Integration y Security; Integration después del refactor: 75 % | listo |
| Semgrep | 0 hallazgos | 0 en los dos escaneos del pipeline (registro y `.semgrep/semgrep.yaml`) | listo |
| SDD | `check_sdd` OK | `SDD-OK (54 specs)` | listo |
| Demostración | CSV real de 20–30 cargado (si existe), repetido sin duplicar y visible en `/categorias` | no hay CSV real del piloto todavía | pendiente |
