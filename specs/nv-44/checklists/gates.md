# gates.md — NV-44 Infraestructura de imágenes: S3 y CloudFront

Evidencia local del 2026-11-13. Sin cambios en Go: build, vet, Unit, `golangci-lint`, cobertura y las suites con contenedor no aplican; el CI del PR las corre igual.

| Gate | Criterio | Evidencia | Estado |
|---|---|---|---|
| critic B1-B4 | Sin secretos, sin datos personales, infraestructura revisada, endpoints al día | `critic-guardrails: PASS` | ✅ |
| Endpoints | Sin rutas nuevas | `check_endpoints: OK (100 rutas)` | ✅ |
| SDD | Spec y `Memoria.md` al día | `SDD-OK (63 specs)` | ✅ |
| Formato | `terraform fmt -check -recursive` | sin diferencias | ✅ |
| Validación | `terraform validate` (`init -backend=false`) | `Success! The configuration is valid.` | ✅ |
| Trivy | `trivy config infra/aws-dev` | 0 hallazgos; excepciones con motivo en `imagenes.tf` (AWS-0089 y 0090 registros y versiones, 0132 SSE-S3, 0010 y 0011 WAF y registros de CloudFront) | ✅ |
| Scripts | `bash -n` de `crear.sh`, `destruir.sh` y `verificar.sh` | sin errores | ✅ |
| Bucket | Privado, sin ACL, sin acceso directo, cuarentena fuera de CloudFront | `verificar.sh` tras el `apply` | pendiente del operador |
| `plan` | Solo recursos nuevos; sin reemplazar instancias | `crear.sh --solo-plan` | pendiente del operador |
| Destruir | 0 recursos con el bucket con objetos | `destruir.sh` en la próxima recreación | pendiente del operador |
