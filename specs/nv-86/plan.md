# plan.md — NV-86 Notificaciones por categoría y avisos nuevos

1. Mapeo
- internal/notificaciones (categoría, tipos, `ix_notif_categoria`).
- trabajo nocturno (reseña pendiente, cupón por vencer).
- tarea de 5 min (hora de salir).
- preferencias de notificaciones.
- Copiar las filas de Contratos a `docs/endpoints.md` al ejecutar (no antes: `check_endpoints` compara con el código).

2. Guardarraíles
- Privacidad, propiedad y bloqueos probados completos; mocks solo de interfaces externas; índices por `scripts/indexes`.

3. Secuencia
1. Categoría por tipo y filtro.
2. Tipos nuevos y agrupado.
3. Trabajos programados.
4. Preferencias nuevas.
5. Pruebas, documentos y `Memoria.md`.

4. Verificación
- build, vet, Unit y lint en local; Integration, Security, Database y Contract en local con `--network host` y en el CI del PR.
