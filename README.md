# Wherever Backend

API REST en Go (Fiber) para Wherever, desarrollada con **Spec Driven Development**. Este repositorio está en la etapa de **arquitectura base** (`specs/nv-00…nv-03`): no incluye funcionalidad del producto.

## Estructura SDD
- `.specify/memory/constitution.md`: constitución v1.1.0 (canónica). `constitution.md` en raíz es su espejo.
- Contrato de producto: `Wherever_Endpoints_CRUD.xlsx` (237 endpoints, hojas A1–A7 y J1–J9) + documentos de diseño de la app y del backend. Las specs `nv-04+` toman de ahí sus filas.
- `.specify/templates/`: plantillas de spec, plan, tasks y checklist.
- `specs/nv-00…nv-03/`: `spec.md`, `plan.md`, `task.md` y `checklists/gates.md` de cada spec.
- `AGENTS.md`: cómo se trabaja. `Memoria.md`: estado, decisiones y lecciones. `CHECKLIST_PR.md`: gates de PR.
- `.claude/agents/` (5 sub-agentes) y `.claude/skills/` (skills de apoyo).
- `docs/`: `endpoints.md`, `rate-limit-matrix.md`, `asvs-l2-checklist.md` (vivos).

## Mapa de specs
| Spec | Qué fija |
|---|---|
| `nv-00` | Constitución, stack y gobernanza |
| `nv-01` | Foundation: pipeline, salud, config, errores |
| `nv-02` | Molde DTO → validator → service sin endpoints |
| `nv-03` | Puerta de calidad incremental (advisory) |
| `nv-04` | Autenticación, sesiones y recuperación de contraseña (J1) |
| `nv-05` | Usuarios, perfil, gustos y privacidad (A1) |

Estado: `nv-04` aprobada y `nv-05` ejecutada (Borrador con evidencia); `nv-00…nv-03` en Borrador con evidencia. En `nv-04` falta `POST /auth/google` (espera la decisión D-1).

## Arranque local
1. `cp configs/.env.example .env` y poner un `JWT_KEY` propio de 32 bytes o más.
2. `make infra-up` (o `docker compose -f deployments/docker-compose.yml up -d --wait`). Si un puerto está ocupado, usar `MONGO_HOST_PORT`, `REDIS_HOST_PORT` o `NEO4J_BOLT_HOST_PORT` y ajustar la URI en `.env`.
3. `make indexes` (o `go run ./scripts/indexes`): crea los índices de MongoDB; se puede correr las veces que sea. Las ciudades se cargan con `go run ./scripts/importar_ciudades ciudades.csv` (CSV `nombre,estado,latitud,longitud,con_cobertura` o JSON).
4. `go run ./cmd/api` y probar `curl localhost:8080/ready`. Sin proveedor de correo, los enlaces de recuperar y verificar no se envían: el log solo dice el tipo y el destinatario enmascarado.
5. Pruebas: `make test-unit`; con Docker, `make test-integration`, `make test-security`, `make test-database` y `make test-contract` (una a la vez). `make contract-capture` regenera las capturas de Contract.
