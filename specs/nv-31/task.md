# task.md — NV-31 Interpretar la frase y armar candidatos

## Fase 0 — Preparación
- [ ] T01 Aprobar la spec; rama `nv-31`.

## Fase 1 — Implementación
- [ ] T02 POST /planes/generar recibe una frase de hasta 300 caracteres y devuelve un plan sin…
- [ ] T03 Interpretar (US-041): la IA convierte la frase en un JSON validado: compañía,…
- [ ] T04 Candidatos: el motor de reglas (nv-23) arma una lista de 15 negocios abiertos y…

## Fase 2 — Cierre
- [ ] T05 Pruebas, documentos y `Memoria.md`.
- [ ] T06 `go build`, `go vet`, cinco suites, `critic`, `check_sdd`, `check_endpoints`; PR; evidencia en `gates.md`.
