# Checklist — nv-01 Foundation

> Trazabilidad: `spec R1-R6 ↔ plan §3-§5 ↔ task T1`. Estado: Borrador sin ejecutar.

- [ ] Pipeline en el orden constitucional (R1), verificado por test
- [ ] `/health` 200 y `/ready` con check real de las tres bases (R2)
- [ ] `ping`, versionado estricto, `snake_case`, OpenAPI solo dev, sondas gateadas (R3)
- [ ] Configuración solo con claves usadas; `JWT_KEY` corta falla al arrancar (R4)
- [ ] `NewApp` reutilizable en tests; con Redis caído solo el limitador usa memoria, lo demás fail-closed (R5)
- [ ] Layout sin carpetas de caos, rendimiento ni mutación; `deployments/docker-compose.yml` y `Dockerfile` creados (R6)
- [ ] Tests: Unit 27 + Integration 13 + Security 6 verdes
- [ ] `go build` y `go vet` sin errores
- [ ] `critic PASS`, `endpoints OK`, `semgrep 0`
- [ ] `curl /ready` 200 con las tres bases y 503 sin MongoDB; `/health` trae `estado` y `version`
- [ ] `make infra-up` levanta las tres bases y `docker build .` OK
- [ ] `docs/endpoints.md` con las filas de nv-01
- [ ] Aprobación firmada: (pendiente de revisor)
