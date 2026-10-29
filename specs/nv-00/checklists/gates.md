# Checklist — nv-00 Constitución base

> Trazabilidad: `spec R1-R4 ↔ plan §3-§5 ↔ task T1`. Estado: Borrador sin ejecutar.

- [ ] `specs/nv-00/spec.md` existe con Contexto/Requisitos/Diseño/Contratos/Tests/Criterios/Límites/Aprobación
- [ ] Stack fijado (R1): sin Stripe/FCM/OTel/AWS/hosting
- [ ] Gobernanza SDD + DoD + trazabilidad + 1 revisor (R2)
- [ ] NFR con comando: go build, coverage ≥45% (R3)
- [ ] Reglas no negociables + real-first + sin globals (R4)
- [ ] `AGENTS.md §1` cita el stack + `Memoria.md` existe
- [ ] `configs/.env.example` solo con claves usadas, sin secretos
- [ ] `.golangci.yml`: gosec + errcheck + staticcheck + gocyclo ≤10
- [ ] `ConfigTest (3)` verdes
- [ ] `critic-guardrails.sh → PASS` en vacío
- [ ] Aprobación firmada: (pendiente de revisor)
