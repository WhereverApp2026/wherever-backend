# NV-03 — Puerta de Calidad Incremental (cobertura delta + mutación advisory)

> Deriva de: `.specify/memory/constitution.md §7-§8` + `AGENTS.md §4` + skills `10-testing-strategy`.
> Predecesoras: `nv-01` y `nv-02`. No las modifica. `nv-02` queda reservado al molde.

## 1. Objetivo
Automatizar para cada módulo una puerta de calidad incremental: medir la cobertura del **delta** (mínimo 80% **informativo**) y ejecutar mutación enfocada en lo cambiado (**advisory**), integradas con la suite obligatoria y sin alterar el orden CI canónico.

## 2. Alcance / No-alcance
In: secuencia de 6 capas en modo advisory + diseño de la configuración de mutación (`go-mutesting`) + diseño de `scripts/check_delta_coverage.py --min 80 --advisory` + exclusiones de higiene + checklist de instanciación.
Out: bloqueo por 80% de delta o por mutation score (requiere enmienda §7; hoy el único bloqueo es 45% global); reorden del pipeline; mutación global en el flujo ordinario; rendimiento/caos/Pact real; Terraform/hosting.

## 3. Contratos API
Sin cambio de contrato. `docs/endpoints.md` intacto. Esta puerta solo agrega gates, no rutas.

## 4. Modelo de datos
Sin modelo. Sin colecciones ni índices.

## 5. Seguridad
Sin superficie nueva. La capa Security reutiliza lo existente: `govulncheck` en CI + suite `Security` + `semgrep` al final + `critic B1/B2`. Sin secretos en Git (`B1`), sin `token` en logs (`B2`). Pipeline intacto (`B3`), `endpoints.md` intacto (`B4`).

## 6. Tests / Gates
Secuencia por módulo (orden CI canónico; la puerta corre tras `coverage`):
1. `Security` (suite + govulncheck + semgrep, existentes).
2. Métricas base: `scripts/check_coverage.sh --min 45` (bloqueante, §7).
3. `Unit + Integration` con bases reales (existentes).
4. `Database` con contenedores efímeros e índices (existente).
5. `Contract` con `CONTRACT_CAPTURE` + convención `snake_case` (existente).
6. Puerta incremental (nueva, advisory): `check_delta_coverage.py --min 80 --advisory` (delta = `git merge-base main` → `*.go` bajo `internal/`, excluye higiene; informa, exit 0) + `go-mutesting` sobre los archivos cambiados (`continue-on-error`, registra el score).
Higiene (excluida del delta y de la mutación): `cmd/*/main.go`, mocks generados, DTOs anémicos, `testutil`, scripts de índices.

## 7. Plan de entrega
Sin `build` de imagen, sin tag, sin endpoint. Entrega: 4 documentos en Borrador + diseños de la configuración de mutación y de `check_delta_coverage.py`. La implementación (script + config + job CI + piloto medido en 1 slice) es la fase siguiente con su `T2`. Tras la mutación siempre `go build ./...`.

## Aprobación
- [ ] Revisor humano — (pendiente de nombre)
- [ ] Fecha:
- [ ] CHECKLIST_PR.md OK
- [ ] Sin conflicto con constitución
- **Estado:** 🚧 Borrador sin ejecutar
