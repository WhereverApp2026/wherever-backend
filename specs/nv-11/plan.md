# plan.md — NV-11 Datos gestionados externos

1. Mapeo
- Crear: `scripts/siembra` y `internal/platform/neo4j/grafo.go` (esquema y demostración), `scripts/medir-redis.sh` (conteo de comandos por operación), pruebas de clientes, grafo y presupuesto.
- Modificar también: `infra/aws-dev/main.tf` (salidas 6379 y 7687), limitador y `internal/http/app.go` (presupuesto), `docs/endpoints.md`.
- Modificar: `server/compose.yml` (quitar Redis y Neo4j), `deploy-env.sh` solo si hace falta, `infra/aws-dev/scripts/verificar.sh`, `README.md`, `.specify/memory/constitution.md` (§3), `AGENTS.md`, `CHECKLIST_PR.md`, `Memoria.md`.
- Sin cambios: contratos, `docs/endpoints.md`, `ci-pr.yml`, `cd-dev.yml`, Terraform (sin recursos nuevos).

2. Guardarraíles
- Credenciales solo en Parameter Store, cargadas por el operador, sin pasar por el chat ni Git.
- No se quitan los contenedores locales hasta verificar los servicios externos.
- Si la estimación de comandos de Redis pasa del 60 %, no se avanza sin plan B.
- Registro de comandos y de tamaño del grafo antes de cerrar.

3. Pruebas
- Locales: `go build`, `go vet`, cinco suites, `check_sdd`, `critic`.
- Reales: flujo de extremo a extremo, fallo de dependencia, medición de comandos, tamaño del grafo.

4. Secuencia
1. Revisor aprueba (a)–(d).
2. Operador crea las cuentas con el correo del proyecto y las bases en regiones cercanas.
3. Operador carga los parámetros en Parameter Store.
4. Medir comandos de Redis por operación en local y estimar el consumo mensual.
5. Esquema e índices en el Neo4j externo con `/indexes`; siembra.
6. Probar la API local contra los servicios externos.
7. Aplicar en el servidor: nuevo entorno y `compose.yml` sin Redis ni Neo4j.
8. Flujo de extremo a extremo y prueba de fallo.
9. Retirar los contenedores y volúmenes locales tras verificar.
10. Documentos y evidencia; `Memoria.md`.

5. Verificación
- `curl -i https://18-227-230-50.sslip.io/ready → 200`
- `infra/aws-dev/scripts/verificar.sh → exit 0`
- En el servidor, `ss -ltn` sin puertos 6379 ni 7687 escuchando.
