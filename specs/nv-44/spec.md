# NV-44 — Infraestructura de imágenes: S3 y CloudFront

## Contexto
Un bucket privado servido solo por CloudFront, creado con Terraform. **Origen**: TEC-32. **Depende de**: NV-42, NV-12, NV-14.

## Requisitos
1. Bucket privado con bloqueo de acceso público, sin ACL, cifrado y sin listar; solo CloudFront lo lee (control de acceso de origen).
2. Distribución de CloudFront propia para imágenes con caché de un año (nombres inmutables) y HTTPS.
3. La cuarentena (`cuarentena/`) nunca sale por CloudFront y se vacía sola en 24 horas.
4. El rol de la instancia solo escribe, lee y borra en `cuarentena/*` e `i/*`; el boundary pone el mismo techo.
5. `destruir.sh` (nv-14) borra el bucket aunque tenga objetos.

## Diseño
- **Terraform** `infra/aws-dev/imagenes.tf`: bucket `wherever-dev-imagenes-<cuenta>` con `force_destroy`, bloqueo de acceso público, `BucketOwnerEnforced`, SSE-S3, ciclo de vida (`cuarentena/` a 1 día, subidas incompletas a 1 día), política del bucket (CloudFront lee solo `i/*` con `AWS:SourceArn` de la distribución; se niega lo que no sea HTTPS), OAC, política de caché propia de un año y segunda distribución con `Managed-SecurityHeadersPolicy`.
- **Prefijos:** `cuarentena/` para subidas sin moderar e `i/<id>/<tamano>.jpg` para publicadas (nv-45).
- **Permisos:** política en línea `imagenes` en el rol de la instancia; `ImagenesDelProyecto` en `iam/wherever-dev-boundary.json`; política nueva `iam/wherever-dev-deployer-imagenes.json` para quien aplica Terraform (S3 acotado al bucket, sin ACL ni sitio web; CloudFront de la cuenta), porque la del deployer ya no admite más caracteres.
- **Salidas:** `imagenes_bucket` e `imagenes_url`; `verificar.sh --imagenes` comprueba bucket privado, sin ACL, sin acceso directo y cuarentena fuera de CloudFront.
- **Variables:** ninguna en esta spec. nv-45 lee `IMAGENES_BUCKET` e `IMAGENES_URL` de `/wherever/dev/`, opcionales fuera de prod.
- **Costo:** CloudFront dentro de su capa gratuita (1 TB y 10 millones de peticiones al mes); S3 unos US$0.05 al mes para el piloto. Total estimado: unos US$41.35 de US$50.

## Contratos
Sin endpoints nuevos.

## Tests
- `terraform fmt -check`, `terraform validate` y `trivy config infra/aws-dev` sin hallazgos (las excepciones llevan su motivo).
- Tras `apply`, `verificar.sh` (salida `verificar`): bucket con el acceso público bloqueado, sin ACL, 403 por la URL directa de S3 y 403 en `cuarentena/` por CloudFront.
- La subida, publicación y lectura de una imagen real se prueban en nv-45.

## Criterios
- El bucket es privado y solo CloudFront lo sirve.
- `destruir.sh` termina con 0 recursos aunque el bucket tenga imágenes.

## Límites
- La URL de CloudFront de imágenes cambia si se recrea la distribución (nv-14); `crear.sh` la imprime.
- S3 aplica la expiración en un ciclo diario: un objeto de cuarentena puede durar hasta unas 48 h.
- Sin versiones ni registros de acceso del bucket (costo); sin WAF.

## Aprobación y Control de Cambios
- **Estado:** 🚧 Borrador con evidencia (2026-11-13), sin firma; ola 6.
- **Revisores:** Osmar Reséndiz (pendiente de aprobar)
- **Fecha:** —
- **Pendiente del operador:** IAM, `crear.sh` y `verificar.sh` (ver `infra/aws-dev/README.md` y `Memoria.md`).
- **Detalle:** evidencia en `specs/nv-44/checklists/gates.md` + `Memoria.md`.
