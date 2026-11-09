# task.md — NV-42 Evaluación de imágenes y texto

## Fase 0 — Preparación
- [ ] T01 Aprobar la spec; rama `nv-42`.

## Fase 1 — Implementación
- [ ] T02 Paquete moderacion.Evaluar(ctx, Contenido) que devuelve aprobado, cuarentena o…
- [ ] T03 Imágenes: Amazon Rekognition DetectModerationLabels (desnudez, violencia, drogas,…
- [ ] T04 Texto: clasificación con el modelo de nv-26 en español (venta de drogas, contenido…
- [ ] T05 Todo contenido nuevo se publica solo con resultado aprobado; si la moderación falla,…

## Fase 2 — Cierre
- [ ] T06 Pruebas, documentos y `Memoria.md`.
- [ ] T07 `go build`, `go vet`, cinco suites, `critic`, `check_sdd`, `check_endpoints`; PR; evidencia en `gates.md`.
