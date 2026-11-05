# task.md — NV-10 Fuzzing nativo y pruebas de bordes

## Fase 0 — Preparación
- [x] T01 Decisiones (a)–(c) dadas al ordenar ejecutarla; se confirman al firmar.
- [x] T02 Rama `nv-10-fuzzing` desde `main` actualizada.

## Fase 1 — Inventario (Req. 1, 5)
- [x] T03 Listar objetivos de fuzz por módulo en el inventario de `plan.md`.
- [x] T04 Revisar qué cubren las cinco suites y listar los huecos de integración.

## Fase 2 — Fuzz de entradas (Req. 1, 2, 4)
- [x] T05 Fuzz de parsers de payload, DTOs y validaciones.
- [x] T06 Fuzz de validación de JWT (malformados, `alg: none`, firma alterada, claims inesperados).
- [x] T07 Fuzz del análisis y verificación del hash codificado de Argon2id (parámetros bajos).
- [x] T08 Semillas en `testdata/fuzz/` y `go test` las corre.

## Fase 3 — Handlers (Req. 3)
- [x] T09 App real con dependencias caídas, reutilizando `authtest.Build`; sin repositorios simulados.
- [x] T10 Fuzz de handlers de mayor riesgo con `app.Test()`; verificar invariantes.

## Fase 4 — Bordes de integración (Req. 5)
- [x] T11 Mongo caído o lento → `/ready` 503 y API sin pánico.
- [x] T12 Redis caído o lento → comportamiento controlado según el diseño vigente (fail-closed donde la constitución lo exige).
- [x] T13 Neo4j caído o lento → error controlado.
- [x] T14 Payloads malformados de extremo a extremo y tiempos de espera; reintentos solo donde existan.

## Fase 5 — Nocturno (Req. 4)
- [x] T15 Job `fuzz` en `nightly.yml` con `-fuzztime` acotado y artefacto de fallos.
- [x] T16 Corrida manual dentro del tiempo previsto.

## Fase 6 — Cierre
- [x] T17 Corregir hallazgos; documentar en `docs/endpoints.md` si cambia algún error (critic B4).
- [x] T18 Verificar que la cobertura no baja del 45 %.
- [x] T19 `AGENTS.md`, `Memoria.md`.
- [x] T20 `go build`, `go vet`, cinco suites, `check_sdd`, `critic`; PR; evidencia en `gates.md`.
