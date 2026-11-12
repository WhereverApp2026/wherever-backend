# plan.md — NV-51 Búsqueda semántica y evaluación

1. Mapeo
- Crear: `internal/semantica/{indice,repository,service,dto,handler}.go` y su prueba Unit; `tests/integration/semantica_test.go`; `scripts/evaluar-semantica/{main.go,conjunto.json}`; `docs/ml-evaluacion.md`.
- Modificar: `internal/recomendador` (factor `afinidad_semantica`, pesos, explicación); `internal/asistente/service.go` (`WithSemantica`); `internal/embeddings/armar.go` (`Vectorizador` compartido); `internal/negocios/candidatos.go` (`CardOf`); `cmd/api/main.go`, `internal/testutil/authtest`, `scripts/routes/main.go` (módulo antes que negocios); `docs/endpoints.md`, `docs/rate-limit-matrix.md`; `tests/integration/recomendador_test.go` (puntajes con los pesos nuevos).
- Sin cambios: colecciones, índices, ARCO y exportación (la caché no lleva usuario), `.env.example` (sin variables nuevas).

2. Guardarraíles
- Servicios reales en las pruebas (Gemini es un servidor local), sin secretos en Git, sin la frase en logs ni en llaves de Redis, cada ruta nueva en `docs/endpoints.md` y en `scripts/routes`.
- El texto vectorizado es solo público; la frase va anonimizada.

3. Secuencia
1. Preparación: rama `nv-51` desde `main` (ya trae nv-50, #53).
2. Implementación: factor y pesos → paquete `semantica` → cableado → pruebas → script de evaluación.
3. Cierre: documentos, `Memoria.md`, gates y PR.

4. Verificación
- Local: `go build`, `go vet`, Unit, `golangci-lint`, `critic`, `check_endpoints`, `check_sdd`.
- CI del PR: Integration, Security, Database, Contract, cobertura ≥ 45 % y Semgrep en 0.
