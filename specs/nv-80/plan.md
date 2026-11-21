# plan.md — NV-80 Regla y vigencia del Sello de Excelencia

1. Mapeo
- internal/negocios/panel (sello.go, resumen, insignia).
- config `SELLO_VIGENCIA_MESES`.
- trabajo nocturno de nv-56.
- ruta pública `GET /sello`.
- Copiar las filas de Contratos a `docs/endpoints.md` al ejecutar (no antes: `check_endpoints` compara con el código).

2. Guardarraíles
- Privacidad, propiedad y bloqueos probados completos; mocks solo de interfaces externas; índices por `scripts/indexes`.

3. Secuencia
1. Criterios de la maqueta con tasa de respuesta.
2. Vigencia, vencimiento nocturno y avisos.
3. `vigente_hasta` en insignia y panel.
4. `GET /sello` con los criterios.
5. Pruebas, documentos y `Memoria.md`.

4. Verificación
- build, vet, Unit y lint en local; Integration, Security, Database y Contract en local con `--network host` y en el CI del PR.
