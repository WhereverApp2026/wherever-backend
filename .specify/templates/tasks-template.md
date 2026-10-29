# Tasks Template — SDD

> Cada módulo liberable: spec→índices→DTO→servicio→handler→tests→gates→tag.

- [ ] 1. Spec en `specs/nv-0X/spec.md` aprobada
- [ ] 2. Colección + índices por script idempotente
- [ ] 3. DTO + validator, JSON `snake_case`
- [ ] 4. Servicio por constructor, sin globals; `Reset()` + mutex si hay estado
- [ ] 5. Handler `/api/v1/...` sin `recover` ad-hoc, mapeo 404/403/409/422
- [ ] 6. Tests: Unit + Integration real + Security + Database + Contract
- [ ] 7. Gates: cobertura ≥45% + `endpoints.md` + `rate-limit-matrix.md` + `asvs-l2-checklist.md` vivos
- [ ] 8. `go build` + imagen + curl `/ready` + tag `vX.Y.Z-modN`
- [ ] 9. Actualizar `Memoria.md` + `CHECKLIST_PR.md`
