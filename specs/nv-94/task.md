# task.md — NV-94 Restablecer la contraseña con código

## Fase 0 — Preparación
- [x] T00 Spec en Borrador; rama `nv-94-codigo-restablecer` contra `main`.

## Fase 1 — Implementación
- [x] T01 `recuperar` manda un código de 6 dígitos con tope de 5 en 24 h.
- [x] T02 `restablecer` con `{correo, codigo, password_nuevo}`, 5 intentos y 423; correo sin cuenta = mismo 400.
- [x] T03 Plantilla del correo con el código; sin ruta `/restablecer` en los enlaces.

## Fase 2 — Cierre
- [x] T04 Pruebas en las suites que apliquen; OpenAPI; `docs/endpoints.md`; `AGENTS.md`; `Memoria.md`.
