# gates.md — NV-42 Evaluación de imágenes y texto

| Gate | Criterio | Evidencia | Estado |
|---|---|---|---|
| critic B1 | Sin secretos nuevos en Git ni Terraform | `critic-guardrails: PASS` (2026-11-13); `rekognitiontest` usa credenciales fijas y falsas; sin variables nuevas | ✅ |
| critic B2 | Sin contenido ni datos personales en logs | el log solo lleva `parte` y el tipo de error; `TestTexto_ModeloCaidoDejaEnCuarentenaSinRegistrarElTexto` | ✅ |
| critic B3 | Cambios de infraestructura aprobados por el revisor | una política en línea en el rol (`main.tf`) y una sentencia en el boundary y en el deployer, solo `rekognition:DetectModerationLabels` en `us-east-2`; `terraform fmt -check` limpio. Pendiente: aprobación del revisor y aplicarlo antes de nv-45 | pendiente (revisor) |
| critic B4 | Endpoints coinciden con `docs/endpoints.md` | `check_endpoints: OK (100 rutas)`; sin rutas nuevas | ✅ |
| Pruebas | Cinco suites verdes | Local: build, vet, Unit (`internal/moderacion`: Umbrales, Texto, Imagen) y `golangci-lint` v2.15.0. Las demás suites en el CI del PR (sin pruebas nuevas en ellas) | pendiente (CI) |
| Cobertura | Mínimo 45 % | `check_coverage` en el CI del PR | pendiente (CI) |
| Semgrep | 0 hallazgos | lo corre el CI del PR | pendiente (CI) |
| SDD | `check_sdd` OK y `Memoria.md` actualizado | `SDD-OK (63 specs)`; sección NV-42 en `Memoria.md` | ✅ |
| Falla cerrada | Sin resultado no se publica | `TestImagen_ServicioCaidoDejaEnCuarentena`, `TestUmbrales_FuenteCaidaDejaEnCuarentena`, `TestTexto_LargoOSinServicio` | ✅ |
| Umbrales | 40 y 80 con bordes | `TestUmbrales_Bordes` | ✅ |
| Costo | Dentro de US$50 | US$0.001 por imagen; unas 300 fotos al mes = US$0.30 (ver spec) | ✅ |
| En vivo | 5 imágenes y 3 textos | `TestVivo_Moderacion` pendiente: el deployer no tiene el permiso hasta que un administrador suba la política nueva (AccessDenied comprobado el 2026-11-13) | pendiente |
