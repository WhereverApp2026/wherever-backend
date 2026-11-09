# task.md — NV-44 Infraestructura de imágenes: S3 y CloudFront

## Fase 0 — Preparación
- [ ] T01 Aprobar la spec; rama `nv-44`.

## Fase 1 — Implementación
- [ ] T02 Bucket privado con bloqueo de acceso público; solo CloudFront lo lee (control de acceso…
- [ ] T03 Distribución de CloudFront propia para imágenes con caché de un año (nombres inmutables)…
- [ ] T04 El cuarentena se vacía solo en 24 horas.

## Fase 2 — Cierre
- [ ] T05 Pruebas, documentos y `Memoria.md`.
- [ ] T06 `go build`, `go vet`, cinco suites, `critic`, `check_sdd`, `check_endpoints`; PR; evidencia en `gates.md`.
