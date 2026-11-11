# task.md — NV-28 Límite por usuario y protección de datos en la IA

## Fase 0 — Preparación
- [x] T01 Rama `nv-28` desde `nv-27`; drift corregido (`usos_ia`, integración con `ia.Guardia`, 5 generaciones por hora). Falta la firma.

## Fase 1 — Implementación
- [x] T02 Límite por usuario y por hora (`IA_LIMITE_HORA`, 5 por defecto) con `LimiteUsuarioError` (`Regreso`, `RetryAfter()`) y registro en `usos_ia` (`internal/ia/limite.go`).
- [x] T02b Límite global `IA_LIMITE_MIN` (12) e `IA_LIMITE_DIA` (400, día del Pacífico) en Redis, cerrado ante fallos.
- [x] T03 Defensa contra inyección: frase solo en el turno de usuario; `ia.SoloDeLaLista` para los ids.
- [x] T04 Sin datos personales hacia el modelo: frase anonimizada, sin ids de usuario (prueba en `Security/IA`).

## Fase 2 — Cierre
- [x] T05 Pruebas (Unit, `Integration/Límite`, `Security/IA`), `docs/rate-limit-matrix.md`, `.env.example` y `Memoria.md`.
- [x] T06 `go build`, `go vet`, Unit, `golangci-lint`, `critic`, `check_sdd`, `check_endpoints`; PR contra `nv-27`; evidencia en `gates.md`. Integration, Security, Database y Contract en el CI del PR.
