# gates.md — NV-45 Subida, validación y publicación de imágenes

| Gate | Criterio | Evidencia | Estado |
|---|---|---|---|
| critic B1 | Sin secretos nuevos en Git ni Terraform | `critic-guardrails: PASS` (Docker, 2026-11-13) | ok |
| critic B2 | Sin tokens ni datos personales en logs | `critic-guardrails: PASS`; los logs llevan el id aleatorio de la imagen, no el dueño | ok |
| critic B3 | Cambios de infraestructura aprobados por el revisor | `hop_limit` 2 en `infra/aws-dev/main.tf`; espera la firma | pendiente de firma |
| critic B4 | Endpoints coinciden con `docs/endpoints.md` | `check_endpoints: OK (114 rutas)` | ok |
| Local | build, vet, Unit, `golangci-lint` v2.15.0 | Docker `golang:1.27.3`: verde; lint `0 issues` | ok |
| Pruebas | Integration, Security, Database, Contract | CI del PR | en CI |
| Cobertura | Mínimo 45 % | CI del PR | en CI |
| Semgrep | 0 hallazgos | CI del PR | en CI |
| SDD | `check_sdd` OK y `Memoria.md` actualizado | `SDD-OK (65 specs)` | ok |
| EXIF | Foto publicada sin metadatos | `TestProcesar_QuitaEXIFyGPSYRespetaLaOrientacion`; en vivo tras desplegar | Unit ok, vivo pendiente |
| Moderación | Sin aprobación no se sirve | `TestImagenes_ModeracionRechazaODudosa`, `TestImagenes_ColaDeRevisionPublicaODescarta` | en CI |
