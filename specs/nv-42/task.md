# task.md — NV-42 Evaluación de imágenes y texto

## Fase 0 — Preparación
- [x] T01 Revisar drift y costo; rama `nv-42` (la firma queda pendiente).

## Fase 1 — Implementación
- [x] T02 `moderacion.Moderador.Evaluar(ctx, Contenido)` con aprobado, cuarentena o rechazado y puntajes por filtro.
- [x] T03 Imágenes: `moderacion.Rekognition` (`DetectModerationLabels`) y el doble `rekognitiontest`.
- [x] T04 Texto: `moderacion.TextoIA` sobre `ia.Modelo` más palabras bloqueadas y datos personales.
- [x] T05 Falla cerrada: cualquier error deja en cuarentena; IAM en boundary, rol y deployer.

## Fase 2 — Cierre
- [x] T06 Pruebas Unit, README de `infra/aws-dev` y `Memoria.md`.
- [ ] T07 Local verde (build, vet, Unit, lint, critic, `check_sdd`, `check_endpoints`); PR; CI del PR. Pendiente: prueba en vivo tras aplicar IAM.
