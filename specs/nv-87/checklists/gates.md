# gates.md — NV-87 Mensajes directos

Evidencia local del 2026-11-23, en Docker `golang:1.27.3`. Las suites completas las corre el CI del PR.

| Gate | Criterio | Evidencia | Estado |
|---|---|---|---|
| critic B1–B4 | Sin secretos; rutas en `docs/endpoints.md` | `critic-guardrails: PASS`; `check_endpoints: OK (281 rutas)` | ✅ |
| Build, vet y lint | Sin errores; 0 hallazgos | `go build ./...` y `go vet ./...` sin errores; golangci-lint v2.15.0 `0 issues` | ✅ |
| Unit | Reglas puras de la spec | `go test ./internal/... ./cmd/...` verde; `internal/chat/directos_test.go` (DTOs y adjuntos, un mensaje guarda solo `grupo_id` o `conversacion_id`, par ordenado, vistas de mensaje y conversación, 404 del módulo dueño = 422 del adjunto, 401 sin sesión) | ✅ |
| Integration | Casos de la sección Tests | `tests/integration/directos_test.go` (abrir o reutilizar; páginas, `despues_de` e `id_cliente`; no leídos y leído; adjuntos lugar y plan con «enviada»/«aceptada»; ajenos = 422; reacciones y borrar; bloqueo; sin amistad no se escribe; aviso agrupado y silencio; moderación por reglas; 429 a los 120; ARCO) + `TestChat_*` de nv-57 en local con `-run`: ok | ✅ local, CI pendiente |
| Security | 401/403, propiedad, privacidad y límites | `tests/security/directos_security_test.go` (las 7 rutas 401/403; no participante = 404 igual que inexistente y sin texto; sin amistad = 404; borrar lo ajeno = 404; adjunto ajeno = 422; texto fuera de los logs) + `TestChatSecurity_*`: ok | ✅ local, CI pendiente |
| Database | Índices nuevos (si los hay) | `ux_conversacion_par`, `ix_conversacion_participante`, `ix_mensajes_conversacion`, `ux_id_cliente_conversacion` en `tests/database/indexes_test.go`: ok. **Crearlos en Atlas antes del despliegue.** | ✅ local, CI pendiente |
| Contract | Formas nuevas capturadas | `testdata/directos/*` (10 capturas: abrir 201/200, mensaje 201, lista, mensajes, reacción, leído 204, adjunto 422, borrar 204, 404) y `usuarios/exportacion_descargar_200` con `conversaciones`: ok | ✅ local, CI pendiente |
| Redis | Sin costo nuevo | El límite de 120/h y los avisos van en MongoDB; ninguna ruta nueva cachea | ✅ |
| SDD | `check_sdd` OK y `Memoria.md` al día | `SDD-OK (91 specs)`; `Memoria.md` con la entrada de NV-87 | ✅ |
