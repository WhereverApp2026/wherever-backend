# gates.md — NV-10 Fuzzing nativo y pruebas de bordes

| Gate | Criterio | Evidencia | Estado |
|---|---|---|---|
| critic B1 | Sin secretos en semillas ni pruebas | salida de `critic` | pendiente |
| critic B2 | Sin tokens reales en logs ni semillas | salida de `critic` | pendiente |
| critic B3 | Orden de middlewares HTTP intacto (no revisa `nightly.yml`) | salida de `critic` | pendiente |
| critic B4 | Endpoints sin cambios, o fila actualizada si cambió un error | salida de `critic` | pendiente |
| Cobertura | No baja de 45 % | run 41993911344 del PR #16 | OK: 83,4 % (delta 80,6 %) |
| Inventario | Objetivos y huecos listados en `plan.md` | `plan.md` | OK: 11 objetivos, 6 huecos cubiertos |
| Semillas | `go test` pasa con las semillas | Unit: 575 pruebas y subpruebas | OK |
| Detecta | Error plantado encontrado por fuzz o semillas | prueba local, ver abajo | OK: pánico con `{"correo":""}`, encontrado por `FuzzHandlers` en 46 s; las semillas no lo veían |
| Bordes | Mongo, Redis y Neo4j caídos cubiertos | `tests/integration/bordes_test.go` | OK: caídos y lentos |
| Nocturno | Job `fuzz` dentro del tiempo previsto | run 41993909194 | OK: 15 min 6 s, 11 objetivos sin fallos, artefacto `fuzz`. `FuzzHandlers` da solo 6 077 ejecuciones en 60 s con 2 CPU |
| `check_sdd` | Sin errores | salida | pendiente |

## Evidencia local (2026-11-05, Go 1.27.3)
- Suites: Unit 575, Integration 105, Security 46, Database 4, Contract 3; sin fallos. Incluyen subpruebas.
- Fuzz real, 30 s por objetivo, en contenedor con 8 CPU:

| Objetivo | Ejecuciones | Resultado |
|---|---|---|
| `FuzzParseHash` | 719 839 | Sin fallos |
| `FuzzParseRows` | 509 260 | Sin fallos |
| `FuzzParseToken` | 811 253 | Sin fallos |
| `FuzzTokenClaims` | 476 203 | Sin fallos |
| `FuzzBuildMIME` | 692 790 | Sin fallos |
| `FuzzLinkToken` | 1 072 884 | Sin fallos |
| `FuzzDecodeCursor` | 989 466 | Sin fallos |
| `FuzzNormalize` | 636 137 | Sin fallos |
| `FuzzHandlers` | 94 791 | Sin fallos |
| `FuzzDTOs` | 867 471 | Sin fallos |
| `FuzzNormalizePhone` | Falló | **Hallazgo:** `"0\n("` dejaba el salto de línea. Corregido; semilla en `testdata/fuzz/`; 30 s más sin fallos |

- Hash: antes de corregirlo, `t=0` y `p=0` causaban pánico y `m=4294967295` terminó con salida 137 (memoria limitada a 512 MB).
- `FuzzHandlers` antes del fuzz, con 3 cuerpos por ruta: 200 ×6, 400 ×14, 401 ×54, 422 ×5, 503 ×11. Llega a los handlers.
- Tiempos con base lenta: `/ready` hasta 2,96 s (Redis); rutas unos 2,5 s; tiempo máximo de 200 ms respetado.
