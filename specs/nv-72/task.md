# task.md — NV-72 Entrar con Google

## Fase 0 — Preparación
- [x] T01 D-1 decidida por el revisor (camino B, 2026-11-18); spec ajustada. El visto bueno del responsable de A1 queda pendiente en el PR.
- [x] T02 Excepción de stack aprobada y registrada en `Memoria.md`; `GOOGLE_CLIENT_ID` en config y `.env.example`. Parameter Store: lo carga el operador (opcional; sin ella la ruta da 503).

## Fase 1 — Implementación
- [x] T03 `GoogleVerifier` con `aud`, `iss`, `exp`, `email_verified` y 401 uniforme; 503 si no hay llaves.
- [x] T04 POST /auth/google: entrar por `google_id`, vincular por correo (borrar contraseña y revocar sesiones si no estaba verificado), 409 con otro `google_id`.
- [x] T05 Cuenta nueva según D-1 (B: 422 `registro_incompleto` sin guardar nada; con datos, mismas reglas que el registro) y política `auth_google`.

## Fase 2 — Cierre
- [x] T06 Pruebas, `docs/endpoints.md`, `docs/rate-limit-matrix.md` y `Memoria.md`.
- [ ] T07 `go build`, `go vet`, Unit, lint, `govulncheck`, `critic`, `check_sdd`, `check_endpoints` en local; Integration, Security, Database y Contract en el CI del PR; evidencia en `gates.md`.
  - Local hecho (2026-11-18): build, vet, Unit, golangci-lint y govulncheck. Falta el CI del PR (Integration, Security, Database, Contract, critic, check_sdd, check_endpoints).
