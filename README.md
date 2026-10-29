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

Estado: tareas **sin ejecutar**. La funcionalidad específica tendrá sus propias specs (`nv-04+`).
