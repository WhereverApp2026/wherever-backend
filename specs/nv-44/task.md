# task.md — NV-44 Infraestructura de imágenes: S3 y CloudFront

## Fase 0 — Preparación
- [x] T01 Rama desde `origin/main`; drift de la spec corregido (2026-11-13).

## Fase 1 — Implementación
- [x] T02 Bucket privado con bloqueo de acceso público, sin ACL, cifrado y sin listar; solo CloudFront lo lee (OAC).
- [x] T03 Distribución de CloudFront propia para imágenes con caché de un año y HTTPS.
- [x] T04 La cuarentena se vacía sola en 24 horas y no sale por CloudFront.
- [x] T05 Permisos mínimos del rol de la instancia, boundary y política del deployer para imágenes.
- [x] T06 `destruir.sh` con bucket con objetos (`force_destroy`); `crear.sh` y `verificar.sh` al día.

## Fase 2 — Cierre
- [x] T07 Documentos y `Memoria.md`.
- [x] T08 `fmt`, `validate`, Trivy, `critic`, `check_sdd`, `check_endpoints`; PR; evidencia en `gates.md`.
- [ ] T09 Operador: IAM, `crear.sh` y `verificar.sh` en la cuenta.
