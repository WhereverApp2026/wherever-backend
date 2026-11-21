# task.md — NV-82 Privacidad social

## Fase 0 — Preparación
- [x] T00 Rama `nv-82-privacidad-social` contra `main`; decisiones (1)–(3) tomadas como propone la spec, pendientes de firma.

## Fase 1 — Implementación
- [x] T01 Campos `privacidad.social` con valores por omisión (`internal/usuarios/social.go`).
- [x] T02 Efectos en perfil, insignias, momentos, postales, galería, populares, amigos en el lugar y sugerencias (pestañas en nv-81).
- [x] T03 Restricción de solicitudes por código o contactos (`internal/amistades`).
- [x] T04 Exportación ARCO (sale con el perfil).

## Fase 2 — Cierre
- [x] T05 Pruebas en las suites que apliquen; `docs/endpoints.md`; `Memoria.md`; evidencia en `gates.md`.
