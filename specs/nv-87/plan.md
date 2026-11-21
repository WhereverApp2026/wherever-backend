# plan.md — NV-87 Mensajes directos

1. Mapeo
- colección `conversaciones` (`ux_conversacion_par`).
- internal/chat (mensajes con `conversacion_id`).
- internal/planes (invitación).
- internal/notificaciones (`mensaje_directo`).
- Copiar las filas de Contratos a `docs/endpoints.md` al ejecutar (no antes: `check_endpoints` compara con el código).

2. Guardarraíles
- Privacidad, propiedad y bloqueos probados completos; mocks solo de interfaces externas; índices por `scripts/indexes`.

3. Secuencia
1. Conversaciones entre amigos.
2. Mensajes, adjuntos y reacciones.
3. Leídos y avisos.
4. Bloqueo y ARCO.
5. Pruebas, documentos y `Memoria.md`.

4. Verificación
- build, vet, Unit y lint en local; Integration, Security, Database y Contract en local con `--network host` y en el CI del PR.
