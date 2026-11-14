# task.md — NV-72 Entrar con Google

## Fase 0 — Preparación
- [ ] T01 D-1 decidida (A o B) por el revisor y el responsable de A1; spec ajustada a la decisión.
- [ ] T02 Excepción de stack aprobada y registrada en `Memoria.md`; `GOOGLE_CLIENT_ID` en config, `.env.example` y Parameter Store.

## Fase 1 — Implementación
- [ ] T03 `GoogleVerifier` con `aud`, `iss`, `exp`, `email_verified` y 401 uniforme; 503 si no hay llaves.
- [ ] T04 POST /auth/google: entrar por `google_id`, vincular por correo (borrar contraseña y revocar sesiones si no estaba verificado), 409 con otro `google_id`.
- [ ] T05 Cuenta nueva según D-1 (B: 422 `registro_incompleto` sin guardar nada; con datos, mismas reglas que el registro) y política `auth_google`.

## Fase 2 — Cierre
- [ ] T06 Pruebas, `docs/endpoints.md`, `docs/rate-limit-matrix.md` y `Memoria.md`.
- [ ] T07 `go build`, `go vet`, Unit, lint, `govulncheck`, `critic`, `check_sdd`, `check_endpoints` en local; Integration, Security, Database y Contract en el CI del PR; evidencia en `gates.md`.
