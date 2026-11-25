# task.md — NV-91 Verificación de correo con código

## Fase 0 — Preparación
- [x] T00 Spec en Borrador; rama contra `main`.

## Fase 1 — Implementación
- [x] T01 Código de 6 dígitos con HMAC, intentos y bloqueo.
- [x] T02 `verificar-correo` con JWT y `{codigo}`; `reenviar-verificacion` con límite propio y tope diario.
- [x] T03 Plantilla del correo con el código.

## Fase 2 — Cierre
- [x] T04 Pruebas en las suites que apliquen; `docs/endpoints.md`; `AGENTS.md`; `Memoria.md`.
