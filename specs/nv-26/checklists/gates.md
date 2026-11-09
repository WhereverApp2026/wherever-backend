# gates.md — NV-26 Cliente de IA (Gemini) y salida validada

| Gate | Criterio | Evidencia | Estado |
|---|---|---|---|
| critic B1 | Sin secretos nuevos en Git ni Terraform | `critic-guardrails: PASS`; `.env.example` con `GEMINI_API_KEY` vacía y su prueba detecta `AIza…` | listo |
| critic B2 | Sin tokens ni datos personales en logs | `critic-guardrails: PASS`; el adaptador no registra y el error no lleva el cuerpo; `TestLogger_LlaveDeGoogleDentroDeUnTextoSeOculta` | listo |
| critic B3 | Cambios de infraestructura o pipeline aprobados por el revisor (si aplica) | no aplica: sin cambios de IAM ni Terraform | listo |
| critic B4 | Endpoints coinciden con `docs/endpoints.md` | `check_endpoints: OK (38 rutas)`, sin rutas nuevas | listo |
| Pruebas | Cinco suites verdes con servicios reales | Unit verde en `golang:1.27.3` (2026-11-09); faltan las demás | parcial |
| Cobertura | Mínimo 45 % | `check_coverage` | pendiente |
| Semgrep | 0 hallazgos | 0 hallazgos en `internal/ia`, `internal/platform` y `configs` (reglas del pipeline) | listo |
| SDD | `check_sdd` OK y `Memoria.md` actualizado | `SDD-OK (54 specs)`; `Memoria.md` con la sección NV-26 | listo |
| Red | Las instancias salen por 443 a `generativelanguage.googleapis.com` | regla `egress_ports.https` de `main.tf` | listo |
| En vivo | 20 frases con la llave real: latencia p50 y p95, tasa de 429, inválidas | 2026-11-09, `gemini-3.5-flash-lite`, llave desde SSM, 6 s entre llamadas: 20 de 20 válidas, 0 429 (0 %), 0 inválidas; p50 870 ms, p95 1169 ms, máx 1336 ms; 2973 tokens de entrada y 1302 de salida (unos 149 y 65 por frase). La frase con instrucciones (17) dio una salida neutra | listo |
