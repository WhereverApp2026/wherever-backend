# task.md — NV-75 Consejo del día y descripción con IA para el negocio

## Fase 0 — Preparación
- [ ] T01 Spec revisada y aprobada; nv-74 en `main`; rama de trabajo contra `main`.
- [ ] T02 `consejos_negocio` con `ux_consejo_dia` y TTL en `scripts/indexes` y `/ready`; `IA_CONSEJO_DIA` en config y `.env.example`.

## Fase 1 — Implementación
- [ ] T03 Motor de reglas y plantillas del consejo.
- [ ] T04 GET /negocios/{id}/consejo: uno por día, candado por índice único, cupo diario, IA con esquema y respaldo.
- [ ] T05 POST /negocios/{id}/ia/descripcion: validación de 3 palabras, IA con esquema, plantilla y 429 del límite del usuario.
- [ ] T06 Baja del negocio borra sus consejos.

## Fase 2 — Cierre
- [ ] T07 Pruebas (incluida la de privacidad del contexto), `docs/endpoints.md`, estimación de Redis y de Gemini en `Memoria.md`.
- [ ] T08 `go build`, `go vet`, Unit, lint, `critic`, `check_sdd`, `check_endpoints` en local; suites con contenedor en el CI del PR; evidencia en `gates.md`.
