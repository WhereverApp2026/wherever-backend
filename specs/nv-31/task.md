# task.md — NV-31 Interpretar la frase y armar candidatos

## Fase 0 — Preparación
- [x] T01 Rama `nv-31` desde `origin/nv-29`; drift corregido (itinerario con IA a nv-32, sin `docs/ia.md`, sin exportar nada nuevo en `recomendador`). Falta la firma.

## Fase 1 — Implementación
- [x] T02 `POST /planes/generar` (frase 1–300, ubicación, `fecha`, `personas`, `movilidad`) devuelve un plan sin guardar con `fuente`, `aviso` y `parametros`.
- [x] T03 Interpretar (US-041): instrucciones fijas embebidas, esquema JSON, `GenerarValidado` y `Interpretacion.Validar`; frase anonimizada.
- [x] T04 Candidatos: `Recomendar` con 15 resultados, alergias y dieta del perfil; `Armar` con hasta 3 paradas y `Recalcular`.

## Fase 2 — Cierre
- [x] T05 Pruebas Unit, Integration y Security; `docs/endpoints.md` y `Memoria.md`.
- [ ] T06 Local: build, vet, Unit, lint, `critic`, `check_sdd`, `check_endpoints` (hecho); CI del PR: Integration, Security, Database, Contract, cobertura y Semgrep (pendiente).
