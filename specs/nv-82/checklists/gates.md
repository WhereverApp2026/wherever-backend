# gates.md — NV-82 Privacidad social

Evidencia local del 2026-11-21 en Docker `golang:1.27.3` (Windows); el CI del PR manda.

| Gate | Criterio | Evidencia | Estado |
|---|---|---|---|
| critic B1–B4 | Sin secretos; rutas en `docs/endpoints.md` | `critic-guardrails: PASS` | ✅ |
| Build, vet y lint | Sin errores; 0 hallazgos | `go build ./...` y `go vet ./...` limpios; `golangci-lint` v2.15.0: `0 issues` | ✅ |
| Unit | Reglas puras de la spec | `internal/usuarios/social_test.go` (valores por omisión, desconocidos, `nadie`, interruptores), `internal/usuarios/account/social_test.go` (enums, respuesta, `$set` parcial), `internal/amistades` (`allowed`, `origen`), `internal/personas` (`PhoneHash`), `internal/shared/apperr` (`WithCode`) | ✅ |
| Integration | Casos de la sección Tests | `tests/integration/privacidad_social_test.go`: valores por omisión y PATCH parcial, perfil e insignias, momentos/postal/galería, solicitudes por código y contactos, sugerencias, populares, cuenta previa; más las pruebas vecinas de momentos, postales, populares, perfil, sugerencias y amistades: `ok` (70,8 s) | ✅ |
| Security | 401/403, propiedad, privacidad y límites | `tests/security/privacidad_social_security_test.go`: matriz por ruta con `actividad: nadie` (amiga, extraña y sin sesión) y `origen` falso sin saltar `solicitudes=codigo`, sin filtrar código ni teléfono; más las de amistades, personas, postales y grafo: `ok` (56,2 s) | ✅ |
| Database | Índices nuevos (si los hay) | Sin índices nuevos; filtros `$ne: false` sobre `privacidad.social.*` en consultas por `_id` | N/A |
| Contract | Formas nuevas capturadas | `usuarios/privacidad_social_200`, `usuarios/privacidad_social_422`; `social` agregado en `privacidad_200`, `yo_200`, `yo_patch_200`, `cancelar_eliminacion_200` y `exportacion_descargar_200` (solo se suma `social`); captura con `CONTRACT_CAPTURE=1` y validación sin ella: `ok` | ✅ |
| SDD | `check_sdd` OK y `Memoria.md` al día | `SDD-OK (91 specs)`; `check_endpoints: OK (257 rutas)` | ✅ |
