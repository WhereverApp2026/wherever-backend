# plan.md — NV-89 Reservar todo el plan y permisos al compartir

1. Mapeo
- internal/planes (`POST /planes/{id}/reservas`, edición por integrantes).
- internal/reservas (servicio de reserva por parada).
- internal/grupos (`permiso_plan`, enlace con permiso).
- Copiar las filas de Contratos a `docs/endpoints.md` al ejecutar (no antes: `check_endpoints` compara con el código).

2. Guardarraíles
- Privacidad, propiedad y bloqueos probados completos; mocks solo de interfaces externas; índices por `scripts/indexes`.

3. Secuencia
1. Reservar todo con éxito parcial e idempotencia por parada.
2. Permiso `ver`/`editar` en el enlace y por integrante.
3. Edición del plan por integrantes con `editar`.
4. Pruebas, documentos y `Memoria.md`.

4. Verificación
- build, vet, Unit y lint en local; Integration, Security, Database y Contract en local con `--network host` y en el CI del PR.
