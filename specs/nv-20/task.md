# task.md — NV-20 Negocios aprobados, categorías y carga semilla

## Fase 0 — Preparación
- [ ] T01 Aprobar la spec y las 6 decisiones de `## Aprobación` (pendiente de firma); rama `nv-20` creada.
- [x] T02 Confirmar con el revisor que A5 (CRUD, revisión, documentos y cobros) queda fuera y que no se crea el catálogo de 15 gustos.

## Fase 1 — Modelo e índices
- [x] T03 Crear `internal/negocios/catalog.go` con los 6 giros de A2, nombres de pantalla, dietas, accesibilidad y alérgenos; Unit del catálogo.
- [x] T04 Crear `internal/negocios/model.go` con `Business`, `Direccion`, `Horario`, `VentanaMin`, `Foto`, `Accesibilidad`, `Calificacion`, `Revision`, estados y las extensiones `version`, `horario_min[]`, `alergenos[]`.
- [x] T05 Crear `internal/negocios/repository.go` con `Collection = "negocios"`, los 9 índices de A2/A5, `Upsert` y el filtro de visibilidad (`aprobado` y no eliminado).
- [x] T06 Registrar `negocios.Indexes()` en `internal/indexes/indexes.go` y agregar `negocios.Collection` a `tests/database/indexes_test.go` con `_id_` y los 9 nombres; comprobar idempotencia con `scripts/indexes`.

## Fase 2 — Parser y validación
- [x] T07 Crear `internal/negocios/import.go` con el parser CSV/JSON, encabezado exacto, BOM, filas vacías, tope de 100 y separador `|`.
- [x] T08 Implementar las validaciones de `Diseño` (slug, categoría, precio, México, CP, teléfono, horarios, fotos, listas, calificación) y acumular errores por fila.
- [x] T09 Crear `docs/semilla-negocios.csv` con el encabezado y 2 filas de ejemplo marcadas; Unit del parser y del validador, incluido `FuzzParseCSV`.

## Fase 3 — Importación
- [x] T10 Implementar `Service.Import` idempotente por `slug`: dueño por correo o admin, `qr_checkin` con `crypto/rand`, `revision`, `$set`/`$setOnInsert`, `version++`, calificación/cupones/sello preservados al actualizar.
- [x] T11 Implementar `Handler.importar` para `multipart/form-data` (`archivo`) y `application/json`, con 400/413/422/503, respuesta `{creados, actualizados, errores[]}` y logs solo con conteos.
- [x] T12 Agregar `testutil.ValidTokenFor` y `authtest.CreateAdmin`; montar el módulo en `cmd/api/main.go` y en `authtest`.
- [x] T13 Registrar la ruta admin con `limiter.Window("admin", 200, time.Minute)` y `RequireRoles(RoleAdmin)`; actualizar la fila `Admin` de `docs/rate-limit-matrix.md`.

## Fase 4 — Categorías
- [x] T14 Implementar `Repository.Facets` con `$facet` sobre negocios visibles: 6 giros, etiquetas y servicios con conteo, tope 50 y orden estable.
- [x] T15 Implementar `Handler.categorias` con la respuesta de `Diseño`, `Cache-Control: no-store`, sin `ETag`/304 y 503 si MongoDB falla.
- [x] T16 Registrar `GET /api/v1/categorias` (sin auth, límite global) y agregar las dos filas a `docs/endpoints.md` y las rutas/esquemas a `internal/http/openapi.json`.

## Fase 5 — Pruebas y cierre
- [x] T17 Integration/Semilla 8: alta, reimportación, error parcial, dueños, QR estable, `version`, derivados y preservación de campos de control.
- [x] T18 Integration/Categorías 4 y Security/Catálogo 4: conteos, estados excluidos, `no-store`, 401/403 y admin.
- [x] T19 Contract/Negocios 2 con `CONTRACT_CAPTURE=1`; regenerar solo lo capturado y validar sin la variable.
- [x] T20 `go build`, `go vet`, cinco suites `-count=1`, `critic`, `check_endpoints`, `check_sdd`, cobertura ≥45 %, Semgrep 0; actualizar `Memoria.md` y `gates.md`.
- [ ] T21 Cargar el CSV real de 20–30 negocios en el entorno de demostración (si existe), verificar `GET /categorias`, `/ready` y la idempotencia; no versionar el archivo.
