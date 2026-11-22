# gates.md — NV-81 Perfil con contadores y pestañas

Evidencia local del 2026-11-22 en Docker `golang:1.27.3` (suites con contenedor con `--network host`, socket de Docker y `TESTCONTAINERS_RYUK_DISABLED=true`). El CI del PR manda.

| Gate | Criterio | Evidencia | Estado |
|---|---|---|---|
| critic B1–B4 | Sin secretos; rutas en `docs/endpoints.md` | `critic-guardrails: PASS`; `check_endpoints: OK (260 rutas)` | ✅ |
| Build, vet y lint | Sin errores; 0 hallazgos | `go build ./...`, `go vet ./...`; `golangci-lint` v2.15.0 `0 issues` | ✅ |
| Unit | Reglas puras de la spec | `internal/personas`: `TestVeActividad_SegunActividadYAmistad`, `TestPestanaVisible_InterruptorPorPestana`, `TestContadoresDe_NullSegunPrivacidad`, `TestProfileResponse_ContadoresSiempreVisibles`, `TestNivelDe_TablaDeNv79`; `internal/contadores`: `TestNombres_IgualesALosDeCadaModulo`, `TestClave_EntidadDeLaConstitucion`, `TestInvalidar_NilNoFalla`, `TestDeAmigos_HoyEnLaZona`; `internal/postales`: `TestVisiblesPara_PestanaDelPerfil`; `./internal/... ./cmd/...` verde | ✅ |
| Integration | Casos de la sección Tests | `-run "PerfilContadores\|PerfilPestanas\|PrivacidadSocial\|Personas\|Amistades\|Insignias\|Resenas"` ok: `TestPerfilContadores_PropiosAjenosEInvalidacion` (borrados y ocultas no cuentan, caché e invalidación al quitar la amistad), `TestPerfilPestanas_VisibilidadYPrivacidad` (`amigos`, `todos`, interruptores, `nadie`), `TestPerfilPestanas_Paginacion` (cursor de otra persona = 422), `TestPerfilContadores_ListaDeAmigos` | ✅ |
| Security | 401/403, propiedad, privacidad y límites | `-run "PerfilPestanas\|PrivacidadSocial\|Personas"` ok: `TestPerfilPestanasSecurity_SesionY404Uniforme` (propia, bloqueo, inactiva, inexistente = mismo 404; 401; 403; 422), `TestPerfilPestanasSecurity_PlanesSoloPublicosYSinDatosPrivados`, `TestPerfilPestanasSecurity_Limite429` | ✅ |
| Database | Índices nuevos (si los hay) | Sin índices nuevos; usa `ix_dueno_estado_fecha`, `ix_mis_resenas`, `ix_feed`, `ix_de_estado`, `ix_para_estado`, `ix_estado_cierra` (corre en el CI) | ✅ |
| Contract | Formas nuevas capturadas | `CONTRACT_CAPTURE=1`: `personas/perfil_amigo_200` (`nivel{}`, `contadores`, `insignias_destacadas`, `insignias_total`), `personas/pestana_{momentos,planes,resenas}_200`, `personas/pestana_404`, `personas/amigos_200`, `usuarios/yo_200`; suite completa sin captura: ok (58 s) | ✅ |
| SDD | `check_sdd` OK y `Memoria.md` al día | `SDD-OK (91 specs)`; `Memoria.md` con NV-81 | ✅ |
