# plan.md — NV-90 Datos del negocio que pide la maqueta

1. Mapeo
- internal/negocios (campos, ficha y filtro).
- internal/negocios/alta (pasos 1 y 2).
- internal/negocios/panel (cambios pendientes).
- internal/recomendador (señal de experiencias).
- Copiar las filas de Contratos a `docs/endpoints.md` al ejecutar (no antes: `check_endpoints` compara con el código).

2. Guardarraíles
- Privacidad, propiedad y bloqueos probados completos; mocks solo de interfaces externas; índices por `scripts/indexes`.

3. Secuencia
1. Campos y validación.
2. Captura en el alta y como cambio pendiente.
3. Ficha pública, filtro y recomendador.
4. Pruebas, documentos y `Memoria.md`.

4. Verificación
- build, vet, Unit y lint en local; Integration, Security, Database y Contract en local con `--network host` y en el CI del PR.
