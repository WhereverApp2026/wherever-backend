# NV-02 — Molde DTO→Validator→Service sin endpoints

> Deriva de: `.specify/memory/constitution.md §3-§4` + `AGENTS.md §5` + skill `03-api-vertical-slice`.
> Predecesor: `nv-01` aprobada y firmada. No la modifica.

## 1. Objetivo
Dejar el molde vertical cableado y verificable (`DTO → validator → service`), sin exponer ninguna ruta nueva, para que los módulos de `nv-04+` solo agreguen colección y handler.

## 2. Alcance / No-alcance
In: `internal/shared/pagination (PageResult + cursor) + internal/shared/validation (registro único de validators) + internal/shared/apperr (errores tipados) + un DTO demo sin colección + 1 test de arquitectura`.
Out: ningún handler nuevo, ninguna colección ni repositorio, ningún índice, ninguna fila en `docs/endpoints.md`, ningún `cache:{entidad}`, ningún `recover` en handlers, sin Stripe/FCM/OTel/Terraform/hosting.

## 3. Contratos API
Sin cambio de contrato. `docs/endpoints.md` queda intacto (filas de nv-01). Solo se honra la convención ya declarada: `PageResult{items, siguiente_cursor, limite}`, JSON `snake_case`, `/api/v1/<recurso>` con v1 estricta.

## 4. Modelo de datos
Sin modelo. Sin colecciones ni índices. Colecciones futuras: español plural, `snake_case`, `creado_en/actualizado_en` UTC, `version`.

## 5. Seguridad
Sin superficie nueva. Se reutiliza el `ErrorHandler` de nv-01 (`ValidationError → 422`). Sin secretos en Git (`critic B1`), sin `token` en logs (`B2`). Pipeline intacto (`B3`). Inglés en código, sin pendientes sin issue.

## 6. Tests / Gates
`internal/arch/arch_test.go`: los handlers no importan los drivers de MongoDB/Neo4j, los validators son funciones registradas desde un único punto, ningún handler contiene `recover`, `PageResult` serializa `items/siguiente_cursor/limite`, el cursor inválido → error de validación, `limite` fuera de 1..50 → 422.
Gates: `go build ./... → sin errores` → `go test ./internal/... -count=1` 100% → `critic PASS (B1-B4)` → `check_endpoints OK` → `coverage ≥45%` → `semgrep 0 nuevos`. `CONTRACT_CAPTURE=0`.

## 7. Plan de entrega
Sin `build` de imagen, sin tag, sin `curl` nuevo. Solo `build + tests + gates` en vacío + `Memoria.md` al día. `B4` se mantiene porque no hay handler nuevo que exigir.

## Aprobación
- [ ] Revisor 1 — (pendiente de nombre)
- [ ] Fecha:
- [ ] CHECKLIST_PR.md OK
- [ ] Sin conflicto con constitución
- **Estado:** 🚧 Borrador sin ejecutar
