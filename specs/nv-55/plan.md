# plan.md — NV-55 Alta de negocio por pasos y revisión del administrador

1. Mapeo
- Crear: `internal/negocios/alta/{service,handler,dto}.go`, `internal/negocios/expediente/{model,repository,service,privacy,clabe,rfc}.go`, `internal/testutil/negociostest` (alta), pruebas de las cuatro suites.
- Modificar: `internal/negocios/model.go` (`documentos{ine, comprobante}`, `cuenta_deposito` enmascarada), `scripts/indexes` y los índices declarados de `/ready`; rutas en `internal/http`; `carriesTokens` (lectura de documentos); llaves que oculta el logger; `internal/privacidad` (registro de `Privacy` y retención en el trabajo nocturno); tipo `datos_negocio` en `consentimientos` (nv-19); tipo de notificación `negocio_revisado` (nv-40); `docs/endpoints.md` (filas de A5 a la tabla viva al programar) y `docs/rate-limit-matrix.md` (política «Expediente del negocio»).
- Sin cambios: `/lugares` (solo se prueba que no filtra), variables y Terraform (nv-44 solo sirve `i/*` por CloudFront, así que `privado/` queda fuera; `destruir.sh` vacía el bucket de desarrollo).

2. Guardarraíles
- Servicios reales en las pruebas; S3 local compatible para los objetos; nada sensible en logs, caché ni respuestas públicas.
- Todo dato del expediente pasa por `cifrado`; sin `DATA_KEYS` las rutas responden 503.
- La propiedad del negocio y el rol se validan en el service; la lectura del administrador no entrega nada si la bitácora falla.

3. Secuencia
1. Preparación: decisiones D1–D11 resueltas; nv-45 en `main`; aviso con estos datos y `AVISO_VERSION`.
2. Implementación: alta y pasos, fotos, expediente cifrado (documentos, fiscales, CLABE), consentimiento, enviar a revisión, cola y decisión del admin con bitácora, retención y ARCO.
3. Cierre: pruebas, documentos y `Memoria.md`.

4. Verificación
- `go build`, `go vet`, cinco suites, `critic`, `check_endpoints`, `check_sdd`, cobertura ≥ 45 %, Semgrep en 0.
