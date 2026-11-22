# plan.md — NV-83 Me gusta y comentarios

1. Mapeo
- colecciones `me_gusta` (`ux_me_gusta`) y `comentarios` en `internal/reacciones` (módulo nuevo).
- internal/postales (momentos): `Service.Visible` y `comentarios_total`.
- internal/planes: `Service.Visible`, `me_gusta_total` y `comentarios_total`.
- internal/notificaciones: tipos nuevos y `Agrupador` (`notificaciones_agrupadas`).
- internal/resenas: aviso `resena_util`; internal/amistades: `RelationsWith` y `CommonWith`.
- Copiar las filas de Contratos a `docs/endpoints.md` al ejecutar (no antes: `check_endpoints` compara con el código).

2. Guardarraíles
- Privacidad, propiedad y bloqueos probados completos; mocks solo de interfaces externas; índices por `scripts/indexes`.

3. Secuencia
1. Me gusta idempotente en momentos y planes.
2. Comentarios con respuesta, moderación y borrado.
3. Listas de reacciones con pestañas.
4. Avisos agrupados y ARCO.
5. Pruebas, documentos y `Memoria.md`.

4. Verificación
- build, vet, Unit y lint en local; Integration, Security, Database y Contract en local con `--network host` y en el CI del PR.
- Pruebas: `internal/reacciones/reglas_test.go`, `internal/amistades/amistades_test.go` (`countCommon`), `tests/integration/reacciones_test.go`, `tests/security/reacciones_security_test.go`, `tests/database/indexes_test.go`, `tests/contract/reacciones_contract_test.go`.
