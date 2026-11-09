# task.md — NV-48 Capa de cifrado de campos

## Fase 0 — Preparación
- [ ] T01 Aprobar la spec; rama `nv-48`.

## Fase 1 — Implementación
- [ ] T02 Cifrado por campo con AES-256-GCM para ánimo, alergias y fecha de nacimiento.
- [ ] T03 La llave vive en SSM como cadena segura (/wherever/dev/DATA_KEYS), con identificador de…
- [ ] T04 Cada valor guarda enc:v1:<id_llave>:<nonce>:<texto cifrado>; el nonce es aleatorio de…
- [ ] T05 Rotación: llaves nuevas cifran; las viejas siguen descifrando; un comando re-cifra todo.

## Fase 2 — Cierre
- [ ] T06 Pruebas, documentos y `Memoria.md`.
- [ ] T07 `go build`, `go vet`, cinco suites, `critic`, `check_sdd`, `check_endpoints`; PR; evidencia en `gates.md`.
