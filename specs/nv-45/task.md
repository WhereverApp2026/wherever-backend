# task.md — NV-45 Subida, validación y publicación de imágenes

## Fase 0 — Preparación
- [ ] T01 Aprobar la spec; rama `nv-45`.

## Fase 1 — Implementación
- [ ] T02 POST /imagenes/subida devuelve una URL firmada de subida con límites: JPEG, PNG o WebP,…
- [ ] T03 La imagen cae en un prefijo de cuarentena.
- [ ] T04 Si pasa, la API la vuelve a codificar (quita metadatos como GPS y EXIF) en dos…
- [ ] T05 Al eliminar la cuenta o el negocio se borran sus imágenes.
- [ ] T06 Límite por usuario: 20 subidas por hora.

## Fase 2 — Cierre
- [ ] T07 Pruebas, documentos y `Memoria.md`.
- [ ] T08 `go build`, `go vet`, cinco suites, `critic`, `check_sdd`, `check_endpoints`; PR; evidencia en `gates.md`.
