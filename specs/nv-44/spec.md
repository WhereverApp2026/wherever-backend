# NV-44 — Infraestructura de imágenes: S3 y CloudFront

## Contexto
Un bucket privado servido solo por CloudFront, creado con Terraform. **Origen**: TEC-32. **Depende de**: NV-42, NV-12.

## Requisitos
1. Bucket privado con bloqueo de acceso público; solo CloudFront lo lee (control de acceso de origen).
2. Distribución de CloudFront propia para imágenes con caché de un año (nombres inmutables) y HTTPS.
3. El cuarentena se vacía solo en 24 horas.

## Diseño
- **Terraform** `infra/aws-dev/imagenes.tf`: bucket `wherever-dev-imagenes-<cuenta>`, cifrado, ciclo de vida, política del bucket para CloudFront, segunda distribución.
- **Permisos:** `s3` acotado al bucket del proyecto en el rol de la instancia; `s3`, `cloudfront` (ya parcial) y OAC para quien aplica Terraform.
- **Costo:** dentro de la capa gratuita para el volumen del piloto.

## Contratos
Sin endpoints nuevos.

## Tests
- Las que cubren los requisitos de arriba, con servicios reales.

## Criterios
- El bucket es privado y solo CloudFront lo sirve.

## Límites
- La URL de CloudFront cambia si se recrea la distribución (nv-14).

## Aprobación y Control de Cambios
- **Estado:** 🚧 Borrador (2026-11-08), con hallazgos en `Memoria.md`; ola 6: imágenes y moderación, después de la ruta prioritaria (ver `Memoria.md`)
- **Revisores:** Osmar Reséndiz (pendiente de aprobar)
- **Fecha:** —
- **Decisiones para el revisor antes de ejecutar:** Confirmar el alcance y los valores propuestos en Diseño.
- **Detalle:** la evidencia irá en `specs/nv-44/checklists/gates.md` + `Memoria.md`.
