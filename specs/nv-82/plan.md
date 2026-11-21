# plan.md — NV-82 Privacidad social

1. Mapeo
- internal/usuarios (privacidad.social).
- internal/usuarios/account (PATCH privacidad).
- internal/amistades (origen y solicitudes).
- internal/sugerencias, internal/postales, internal/personas.
- Copiar las filas de Contratos a `docs/endpoints.md` al ejecutar (no antes: `check_endpoints` compara con el código).

2. Guardarraíles
- Privacidad, propiedad y bloqueos probados completos; mocks solo de interfaces externas; índices por `scripts/indexes`.

3. Secuencia
1. Campos `privacidad.social` con valores por omisión.
2. Efectos en perfil, pestañas, feed y sugerencias.
3. Restricción de solicitudes por código.
4. Exportación ARCO.
5. Pruebas, documentos y `Memoria.md`.

4. Verificación
- build, vet, Unit y lint en local; Integration, Security, Database y Contract en local con `--network host` y en el CI del PR.
