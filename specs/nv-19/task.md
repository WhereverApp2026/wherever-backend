# task.md — NV-19 Aceptación de términos y aviso de privacidad

## Fase 0 — Preparación
- [x] T01 Conciliar la spec con lo que ya existe (privacidad de nv-05, versiones de nv-16); rama `claude/vibrant-chebyshev-a26616`. Firma del revisor pendiente.

## Fase 1 — Implementación
- [x] T02 Aceptación de términos (US-003): `POST /api/v1/usuarios/yo/consentimientos` agrega una fila a `consentimientos` y copia la aceptación a `privacidad`; `consentimiento_vigente` en el perfil; ARCO e índice `ix_usuario_fecha`.

## Fase 2 — Cierre
- [x] T03 Pruebas (Unit, Integration, Contract, Database), `docs/endpoints.md`, `docs/aviso-privacidad.md` y `Memoria.md`.
- [x] T04 Local: `go build`, `go vet`, Unit, lint, `critic`, `check_sdd`, `check_endpoints`; PR contra `main`; evidencia en `gates.md`. Las suites con contenedor las corre el CI del PR.
