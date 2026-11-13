# task.md — NV-48 Capa de cifrado de campos

## Fase 0 — Preparación
- [ ] T01 Aprobar la spec (firma pendiente); rama `nv-48` desde `nv-47`.

## Fase 1 — Implementación
- [x] T02 AES-256-GCM por campo con formato `enc:v1:<id>:<nonce>:<cifrado>` y dato asociado colección/campo/id.
- [x] T03 `DATA_KEYS` con llaves por id; la primera cifra y todas descifran.
- [x] T04 Sin llave no arranca en prod; mal formada nunca arranca.

## Fase 2 — Cierre
- [x] T05 Pruebas (unit, fuzz y benchmark), documentos y `Memoria.md`.
- [ ] T06 `go build`, `go vet`, cinco suites, `critic`, `check_sdd`, `check_endpoints`; PR; evidencia en `gates.md`.
