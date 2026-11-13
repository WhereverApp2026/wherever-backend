# task.md — NV-49 Cifrar los datos sensibles en la base

## Fase 0 — Preparación
- [ ] T01 Aprobar la spec (firma pendiente); rama `nv-49` desde `nv-48`.

## Fase 1 — Implementación
- [x] T02 Capa en el repositorio: cifra al escribir y descifra al leer; el resto del código no cambia.
- [x] T03 Fecha de nacimiento cifrada y `mayor_de_edad` al registrar.
- [x] T04 Migración idempotente con modo de prueba (`scripts/migrar-cifrado`), también para rotar.

## Fase 2 — Cierre
- [x] T05 Pruebas, documentos y `Memoria.md`.
- [ ] T06 `go build`, `go vet`, cinco suites, `critic`, `check_sdd`, `check_endpoints`; PR; evidencia en `gates.md`.
- [ ] T07 Cargar `DATA_KEYS` en el entorno y correr la migración (equipo).
