# plan.md — NV-88 Cupo por hora y día del negocio

1. Mapeo
- internal/reservas (cupo y disponibilidad).
- internal/negocios/panel (`reservas_config.cupo_por_hora`, `mesas`, `max_personas`).
- afluencia (nv-66).
- Copiar las filas de Contratos a `docs/endpoints.md` al ejecutar (no antes: `check_endpoints` compara con el código).

2. Guardarraíles
- Privacidad, propiedad y bloqueos probados completos; mocks solo de interfaces externas; índices por `scripts/indexes`.

3. Secuencia
1. Cupo por hora y día con `If-Match`.
2. Disponibilidad con el cupo nuevo.
3. Afluencia y sugerencia por hora.
4. Pruebas, documentos y `Memoria.md`.

4. Verificación
- build, vet, Unit y lint en local; Integration, Security, Database y Contract en local con `--network host` y en el CI del PR.
