# plan.md — NV-81 Perfil con contadores y pestañas

1. Mapeo
- internal/personas (contadores, pestañas).
- internal/usuarios/account (contadores en /yo).
- internal/amistades (planes y planeando ahora).
- caché `cache:perfil_contadores:{usuario}`.
- Copiar las filas de Contratos a `docs/endpoints.md` al ejecutar (no antes: `check_endpoints` compara con el código).

2. Guardarraíles
- Privacidad, propiedad y bloqueos probados completos; mocks solo de interfaces externas; índices por `scripts/indexes`.

3. Secuencia
1. Contadores con caché e invalidación.
2. Pestañas de momentos, planes y reseñas.
3. Nivel e insignias destacadas en el perfil ajeno.
4. Conteos en la lista de amigos.
5. Pruebas, documentos y `Memoria.md`.

4. Verificación
- build, vet, Unit y lint en local; Integration, Security, Database y Contract en local con `--network host` y en el CI del PR.
