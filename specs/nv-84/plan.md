# plan.md — NV-84 Inicio social: planes de amigos e historias

1. Mapeo
- internal/planes (`GET /planes/amigos`, `ix_amigos_feed`): `amigos.go`, `extras.go` (`sharedPlan`), `extras_repository.go` (`FriendsFeed`); `grupos.Repository.GoingTo` y `reacciones.Repository.LikedPlans`.
- internal/postales (`GET /momentos/historias`): `historias.go`, `repository.go` (`Historias`); `resenas.Repository.Validas`.
- internal/votaciones (`GET /votaciones/pendientes`): `pendientes.go`, `repository.go` (`Pending`).
- Pruebas: `tests/integration/inicio_social_test.go`, `tests/security/inicio_social_security_test.go`, `tests/contract/inicio_social_contract_test.go`, `tests/database/indexes_test.go`; Unit en `internal/planes/amigos_test.go`, `internal/postales/historias_test.go`, `internal/votaciones/pendientes_test.go`.
- Copiar las filas de Contratos a `docs/endpoints.md` al ejecutar (no antes: `check_endpoints` compara con el código).

2. Guardarraíles
- Privacidad, propiedad y bloqueos probados completos; mocks solo de interfaces externas; índices por `scripts/indexes`.

3. Secuencia
1. Feed de planes de amigos con visibilidad.
2. Historias de 24 h agrupadas.
3. Votaciones pendientes.
4. Pruebas, documentos y `Memoria.md`.

4. Verificación
- build, vet, Unit y lint en local; Integration, Security, Database y Contract en local con `--network host` y en el CI del PR.
