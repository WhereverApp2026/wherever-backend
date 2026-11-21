# gates.md — NV-79 Insignias, puntos y nivel como en la maqueta

Evidencia local del 2026-11-21 en Docker `golang:1.27.3` (suites con contenedor con `--network host`, socket de Docker y `TESTCONTAINERS_RYUK_DISABLED=true`). El CI del PR manda.

| Gate | Criterio | Evidencia | Estado |
|---|---|---|---|
| critic B1–B4 | Sin secretos; rutas en `docs/endpoints.md` | `critic-guardrails: PASS`; `check_endpoints: OK (257 rutas)` | ✅ |
| Build, vet y lint | Sin errores; 0 hallazgos | `go build ./...`, `go vet ./...`; `golangci-lint` v2.15.0 `0 issues` | ✅ |
| Unit | Reglas puras de la spec | `internal/insignias`: `TestCatalogo_EsElDeLaMaqueta`, `TestDefinicion_GanadaYProgreso`, `TestNivelDe_Bordes` (149/150, 499/500, 2299/2300…), `TestRacha` (huecos, semana actual vacía), `TestRacha_CambioDeAnio` (2026-W53 → 2027-W01), `TestEsperados_ReferenciasYTopeDePlanes`; `./internal/...` verde | ✅ |
| Integration | Casos de la sección Tests | `-run "Insignias\|PrivacidadSocial\|UsuariosService\|Personas\|Usuarios"` ok: `TestInsignias_CatalogoPuntosNivelYRacha` (cada motivo, nivel 4 + siguiente 5/800, racha 3, recrear y borrar no cambian, `GET /usuarios/yo`), `TestInsignias_PlanesTopeSemanalYPerfil`, `TestInsignias_ConcurrenciaSinDuplicados`, `TestInsignias_TareaNocturnaYArco`, `TestInsignias_AjenasYArco`, `TestInsignias_ColeccionesIgualesALosModulos` | ✅ |
| Security | 401/403, propiedad, privacidad y límites | `-run "Insignias\|Personas"` ok; `TestInsigniasSecurity_PuntosAjenos` (ver a otra persona no mueve sus puntos ni expone la bitácora; sin escritura en la ruta) | ✅ |
| Database | Índices nuevos | `ux_movimiento` único en `puntos_movimientos`; suite completa ok | ✅ |
| Contract | Formas nuevas capturadas | `CONTRACT_CAPTURE=1` en `Insignias\|Usuarios`: `insignias/mias_200` (`nivel`, `racha_semanas`), `usuarios/yo_200`, `yo_patch_200`, `cancelar_eliminacion_200` (`nivel_numero`), `exportacion_descargar_200` (`puntos_movimientos`); suite completa sin captura: ok (56 s) | ✅ |
| SDD | `check_sdd` OK y `Memoria.md` al día | `SDD-OK (91 specs)`; `Memoria.md` con NV-79 | ✅ |
