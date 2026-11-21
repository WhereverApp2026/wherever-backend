# plan.md — NV-85 Filtros y tarjetas de Explorar

1. Mapeo
- internal/negocios (filtros y tarjeta).
- índice `ix_negocios_sello`.
- internal/cupones y afluencia (nv-66).
- Copiar las filas de Contratos a `docs/endpoints.md` al ejecutar (no antes: `check_endpoints` compara con el código).

2. Guardarraíles
- Privacidad, propiedad y bloqueos probados completos; mocks solo de interfaces externas; índices por `scripts/indexes`.

3. Secuencia
1. Filtros `con_sello`, `con_cupon`, `de_amigos`, `categorias`, `precio`.
2. Conteo y mapa con los mismos filtros.
3. Tarjeta con Sello, afluencia, cupón y amigos.
4. Pruebas, documentos y `Memoria.md`.

4. Verificación
- build, vet, Unit y lint en local; Integration, Security, Database y Contract en local con `--network host` y en el CI del PR.
