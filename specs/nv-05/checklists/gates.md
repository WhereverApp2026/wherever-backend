# Checklist — nv-05 Usuarios, perfil, gustos y privacidad (A1)

> Trazabilidad: `spec R1-R8 ↔ plan §3-§5 ↔ task T1-T8`. Estado: Aprobada (2026-11-03, Osmar Reséndiz).

- [x] Registro con 18+, política de contraseña, rol, 50 puntos, verificación y tokens (R1) — `TestUsuariosService_RegisterCreatesConsumerWithWelcomePoints`, `TestUsuariosSecurity_AdulthoodByUTCDate`
- [x] `correo-disponible` con límite (R2) — `TestUsuarios_EmailAvailability`, `TestUsuariosSecurity_RegisterAndEmailCheckAreRateLimited`
- [x] Perfil con `version` + `If-Match` (R3) — `TestUsuarios_PatchUsesVersionAndIfMatch` (409 por versión vieja, 400 sin cabecera)
- [x] Cambio de correo (R4) — `TestUsuariosService_ChangeEmailLeavesItUnverified`
- [x] Gustos, preferencias, privacidad, teléfono HMAC (R5) — `TestUsuarios_Tastes`, `TestUsuarios_PartialPreferencesAndPrivacy`, `TestUsuariosService_PhoneIsStoredAsHMACAndUnique`
- [x] Cambio de modo según rol (R6) — `TestUsuarios_ModeChange`
- [x] Autocompletado de ciudades e importación (R7) — `TestCiudades_*` (IXSCAN `ux_ciudad`), `internal/ciudades/service_test.go`
- [x] ARCO: exportar, historial, eliminar, cancelar y trabajo nocturno con candado (R8) — `TestPrivacidad_*`, `TestUsuarios_DeleteClosesSessionsThenCancel`, `TestUsuarios_ExportAndHistory`
- [x] `DELETE /yo/historial` da 503 si falla un módulo y reintentar termina el borrado (idempotente) — `TestRegistry_RetryAfterPartialFailureFinishes`
- [x] Ninguna respuesta con `password_hash`, `telefono_hash` ni `google_id` — `TestUsuariosSecurity_NoSensitiveFieldInResponses`, Contract
- [x] 16 filas en `docs/endpoints.md` (15 + descarga de D-2); `check_endpoints.sh → OK` (37 rutas)
- [x] `go build`, `go vet`, suites, cobertura ≥45 % (82,9 %), semgrep 0
- [x] `Memoria.md` con D-1, D-2, `gocron` y la conexión pendiente de `Purger`/`Exporter`
- [x] Aprobación firmada: Osmar Reséndiz, 2026-11-03

## Evidencia (2026-11-03, Go 1.27.3, Docker 29.8.1, Windows + Git Bash)
- `go build ./...` y `go vet ./...` sin errores; `golangci-lint@v2.15.0 run ./...` → 0 issues.
- Unit → 77 PASS (nuevos: privacidad 5, ciudades 4, usuarios 2, teléfono 1, validación 2, texto 1).
- Integration `-p 1` → 74 PASS: 14 del service de usuarios (`Integration/Usuarios service`) y 4 de privacidad (`usuarios_service_test.go`, con dos instancias a la vez que borran cada cuenta una sola vez), 14 HTTP de usuarios y ciudades (`usuarios_test.go`, con PATCH concurrente: de 4 escrituras con la misma versión gana 1), más los 42 de nv-01/nv-04.
- Security `-p 1` → 26 PASS (10 de usuarios: también el enlace que vence a las 24 h y el archivo que se borra al descargar). Database → 4 PASS con los índices de A1, `ciudades` y `exportaciones`. Contract → 40 capturas (23 nuevas) validadas sin `CONTRACT_CAPTURE`.
- Cobertura unida de las cinco suites (en `golang:1.27.3`): 82,9 %. Delta de nv-05 (advisory): 78,7 % (709/901 líneas); lo que falta son ramas de error de base de datos.
- `critic-guardrails` PASS, `check_endpoints` OK (37 rutas), `check_sdd` SDD-OK, `semgrep 1.95.0` → 0.
- Carrera de registro: 5 POST simultáneos con el mismo correo → 1 × 201 y 4 × 409 (`ux_correo`).
- Trabajo nocturno: borra solo las cuentas vencidas (y sus sesiones), no corre con el candado de otra instancia, suelta el suyo al terminar y el log solo lleva conteos.
