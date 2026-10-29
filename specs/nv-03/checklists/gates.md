# Checklist — nv-03 Puerta incremental

> Trazabilidad: `spec §6 ↔ plan §2-§5 ↔ task T1-T2`. Estado: Borrador sin ejecutar.

- [ ] Las 6 capas documentadas y en el orden canónico
- [ ] `check_coverage.sh --min 45` bloqueante
- [ ] `check_delta_coverage.py --min 80 --advisory` informa y no bloquea
- [ ] Mutación enfocada con `continue-on-error`
- [ ] Higiene (main.go, mocks, DTOs, testutil) fuera del delta
- [ ] Piloto medido y registrado en `Memoria.md`
- [ ] `go build ./...` después de la mutación
- [ ] Aprobación firmada: (pendiente de revisor)
