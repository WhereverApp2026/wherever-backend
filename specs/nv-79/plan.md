# plan.md — NV-79 Insignias, puntos y nivel como en la maqueta

1. Mapeo
- internal/insignias (catálogo, puntos, niveles, racha).
- internal/usuarios (puntos, nivel_numero, racha).
- colección `puntos_movimientos` con `ux_movimiento`.
- trabajo nocturno de conciliación (nv-05).
- Copiar las filas de Contratos a `docs/endpoints.md` al ejecutar (no antes: `check_endpoints` compara con el código).

2. Guardarraíles
- Privacidad, propiedad y bloqueos probados completos; mocks solo de interfaces externas; índices por `scripts/indexes`.

3. Secuencia
1. Catálogo de la maqueta y retiro del de nv-78.
2. Bitácora de puntos idempotente y `$inc`.
3. Niveles y racha.
4. Conciliación al pedir y nocturna.
5. ARCO de `puntos_movimientos`.
6. Pruebas, documentos y `Memoria.md`.

4. Verificación
- build, vet, Unit y lint en local; Integration, Security, Database y Contract en local con `--network host` y en el CI del PR.
