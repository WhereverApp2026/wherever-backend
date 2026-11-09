# plan.md — NV-26 Cliente de IA (Gemini) y salida validada

1. Mapeo
- Crear: `internal/ia/*` (modelo, gemini, esquema, anonimizar, pruebas y `testdata/frases.txt`), `docs/privacidad-ia.md`.
- Modificar: `internal/platform/config` (`GEMINI_API_KEY`, `IA_MODELO_ID`, `IA_TIMEOUT`), `configs/.env.example`, `internal/platform/logger` (oculta llaves de Google en texto libre), `docs/rate-limit-matrix.md`, specs nv-27, nv-28, nv-31, nv-33 y nv-50, `Memoria.md`.
- Sin cambios: IAM y Terraform (Gemini no usa permisos de AWS y la salida 443 ya existe).

2. Guardarraíles
- Mocks solo del proveedor externo (servidor local de `generateContent`).
- Sin secretos en Git, sin datos personales en logs ni hacia el proveedor.
- Instrucciones y entrada del usuario separadas siempre.

3. Secuencia
1. Preparación: decisión del revisor y rama `nv-26`.
2. Implementación: adaptador, validación, anonimización, configuración.
3. Prueba en vivo con la llave real y cierre.

4. Verificación
- `go build`, `go vet`, cinco suites, `critic`, `check_endpoints`, `check_sdd`, cobertura ≥ 45 %, Semgrep en 0.
