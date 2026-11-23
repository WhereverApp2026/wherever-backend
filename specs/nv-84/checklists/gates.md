# gates.md — NV-84 Inicio social: planes de amigos e historias

Evidencia local del 2026-11-22 en Docker `golang:1.27.3` (suites con contenedor con `--network host`, socket de Docker y `TESTCONTAINERS_RYUK_DISABLED=true`). El CI del PR manda.

| Gate | Criterio | Evidencia | Estado |
|---|---|---|---|
| critic B1–B4 | Sin secretos; rutas en `docs/endpoints.md` | `critic-guardrails: PASS`; `check_endpoints: OK (274 rutas)` | ✅ |
| Build, vet y lint | Sin errores; 0 hallazgos | `go build ./...`, `go vet ./...`; `golangci-lint` v2.15.0 `0 issues` (`Friends`, `Historias` y `Pendientes` partidas por gocyclo) | ✅ |
| Unit | Reglas puras de la spec | `internal/planes/amigos_test.go`: `TestSharedWithFriends_AmigosYPublico`, `TestVisibilidad_AceptaAmigos`, `TestNewFriendsWindow_HoyA30DiasYCreados7`, `TestNewFriendPlan_ApuntadosYPrivacidad`; `internal/postales/historias_test.go`: `TestAgruparHistorias_PersonaMasRecientePrimeroYCronologico`, `TestAgruparHistorias_Hasta30Personas`; `internal/votaciones/pendientes_test.go`: `TestPuedeVotarEn_InvitadaConfirmadaSinSorpresa`; `./internal/...` verde | ✅ |
| Integration | Casos de la sección Tests | `-run "TestPlanesAmigos_\|TestHistorias_\|TestVotacionesPendientes_"` ok: `TestPlanesAmigos_VisibilidadVentanaSilencioYPrivacidad` (amigos y público sí; privado, fuera de ventana, no amiga, silenciada, `compartir_planes: false` y `actividad: nadie` no; cursor 2+1 y de otra persona 422; detalle de `amigos` 200 para amiga y 404 para extraña), `TestPlanesAmigos_ApuntadosMeGustaYHacerMiVersion` (me apunto y me gusta en `amigos`, `apuntados` 3 con 1 amigo con nombre, `yo`, sin integrantes/zona/notas; clonar = copia privada), `TestHistorias_AgrupadasUltimas24hYVerificado` (24 h, privada fuera, orden por persona y dentro, verificado por check-in, `compartir_postales: false` fuera), `TestVotacionesPendientes_FaltaTuVoto` (abierta sin voto sí; votada, vencida y cerrada no; declinó y ajena vacía). Antes, `-run "TestReacciones_\|TestPlanes"` sin regresiones | ✅ |
| Security | 401/403, propiedad, privacidad y límites | `-run "TestInicioSocialSecurity_\|TestReaccionesSecurity_\|TestPrivacidadSocialSecurity_\|TestPostalesSecurity_"` ok: `TestInicioSocialSecurity_SesionYCuentaActiva` (3 rutas: 401 y 403), `TestInicioSocialSecurity_SoloLoCompartido` (privado, no amiga y bloqueada no salen ni se leen; historias vacías; `compartir_planes: false` y `actividad: nadie` ocultan feed y detalle; sin integrantes, zona, correo ni teléfono) | ✅ |
| Database | Índices nuevos (si los hay) | `-run TestIndexes_` ok: `planes` con `ix_amigos_feed` `{dueno_id, visibilidad, estado, fecha}` | ✅ |
| Contract | Formas nuevas capturadas | `CONTRACT_CAPTURE=1 -run TestContract_InicioSocial`: `inicio/{planes_amigos_200, planes_amigos_422, historias_200, votaciones_pendientes_200}` | ✅ |
| SDD | `check_sdd` OK y `Memoria.md` al día | `SDD-OK (91 specs)`; `Memoria.md` con NV-84 | ✅ |

Redis: sin cambios (ni claves ni rutas que lo usen), así que no aplica `TestPresupuestoRedis_*`.
