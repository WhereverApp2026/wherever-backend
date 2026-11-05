# plan.md — NV-10 Fuzzing nativo y pruebas de integración de bordes

1. Mapeo
- Crear:
  - `*_fuzz_test.go` junto a cada paquete con entradas externas (objetivos del inventario de este plan).
  - `testdata/fuzz/<Objetivo>/` con casos semilla.
  - `scripts/ci/fuzz.sh`. La app de los fuzzers reutiliza `authtest.Build` con dependencias caídas; sin repositorios simulados (ver spec §Diseño).
  - `tests/integration/bordes_test.go`.
- Modificar: `internal/auth/password.go` (corrección del hallazgo), `internal/testutil` (`testing.TB`), `.github/workflows/nightly.yml` (job `fuzz`), `AGENTS.md`, `Memoria.md`.
- Sin cambios: contratos, `ci-pr.yml`, `infra/`, `docs/endpoints.md` (el fuzz no reveló 5xx por entrada inválida).

2. Guardarraíles
- Se fuzzea código propio; no se fuzzea el cálculo de Argon2id ni bibliotecas externas.
- Parámetros bajos de Argon2id en pruebas de fuzz.
- Invariantes mínimas: sin pánico, sin 5xx por entrada malformada, ningún token o hash inválido aceptado.
- Los fuzzers no usan red externa ni dependencias reales: la app real con las bases caídas.
- Todo fallo hallado se guarda como semilla y se corrige en el mismo PR.
- La cobertura no baja del gate de 45 %.
- Los reintentos solo se prueban donde el código ya los implementa.
- Tiempo nocturno acotado (propuesta: `-fuzztime` 60 s por objetivo, máximo unos 15 min en total).

3. Pruebas
- Locales: `go build`, `go vet`, cinco suites, `go test` con semillas, cada objetivo con unos segundos de `-fuzz`, `check_sdd`, `critic`.
- Reales: error plantado en rama de prueba, corrida manual de `nightly.yml` con el job `fuzz`.

4. Secuencia
1. Revisor aprueba las decisiones (a)–(c).
2. Inventario: recorrer los módulos y listar objetivos de fuzz (parsers, DTOs, validaciones, JWT, hash, handlers de mayor riesgo) y los huecos de integración de las cinco suites. Quedan en este plan.
3. Fuzz de parsers, DTOs y validaciones.
4. Fuzz de JWT y del hash codificado de Argon2id.
5. Fuzz de handlers con repositorios simulados.
6. Pruebas de bordes de integración (Mongo, Redis y Neo4j caídos o lentos; payloads malformados; tiempos de espera).
7. Job `fuzz` en `nightly.yml`.
8. Corregir hallazgos y guardar semillas.
9. Documentos y evidencia en `gates.md`.
10. `Memoria.md`.

5. Verificación
- `go test ./... → semillas pasan`
- `go test -fuzz=<Objetivo> -fuzztime=30s <paquete> → sin fallos` para cada objetivo
- Rama con error plantado: el fuzz o las semillas lo detectan.
- Corrida manual de `nightly.yml`: job `fuzz` dentro del tiempo previsto.

## Inventario de objetivos
| Módulo | Objetivo | Tipo | Invariante |
|---|---|---|---|
| `auth` | `FuzzParseHash` | Hash | Sin pánico; parámetros dentro de los topes; inválido nunca verifica |
| `http/middleware` | `FuzzParseToken` | JWT | Aceptado ⇒ HS256 y firma HMAC correcta |
| `http/middleware` | `FuzzTokenClaims` | JWT | Firma válida con claims malos ⇒ rechazado |
| `shared/pagination` | `FuzzDecodeCursor` | Parser | Aceptado ⇒ reproducible; error ⇒ `ErrInvalidCursor` |
| `shared/text` | `FuzzNormalize` | Validación | Sin espacios de más |
| `platform/mailer` | `FuzzBuildMIME` | Parser | Sin encabezados inyectados |
| `platform/mailer` | `FuzzLinkToken` | Parser | Token en el fragmento; no cambia el destino |
| `ciudades` | `FuzzParseRows` | Parser | Sin pánico con CSV o JSON rotos |
| `usuarios/account` | `FuzzDTOs` | DTO | 10 DTO: sin pánico; error de contrato |
| `usuarios/account` | `FuzzNormalizePhone` | Validación | Idempotente; sin separadores |
| `testutil/authtest` | `FuzzHandlers` | Handlers | 30 rutas reales: sin 5xx salvo 503; formato de error |

Excluidos: `config` y `dotenv` (no reciben entrada externa), `shared/validation` (se ejerce por `FuzzDTOs`), `privacidad` (sin parsers).

## Huecos de integración encontrados
| Escenario | Antes | Ahora |
|---|---|---|
| Mongo, Redis o Neo4j caídos en `/ready` | Cubierto | Sin cambios |
| Mongo caído en auth | Cubierto | Sin cambios |
| Mongo caído en usuarios y ciudades | Hueco | `TestBordes_MongoCaidoOLentoEnUsuariosYCiudades` |
| Cualquier base lenta | Hueco | `TestBordes_ReadyConDependenciaLenta`, `TestBordes_RedisLento` |
| Neo4j caído con negocio activo | Hueco | `TestBordes_Neo4jCaidoNoFrenaLoDemas` |
| Tiempo de espera con base lenta | Hueco | `TestBordes_TiempoDeEspera` |
| Cuerpos hostiles en rutas con sesión | Hueco | `TestBordes_PayloadsHostilesEnTodasLasRutas` |
| Cuerpo sobre el límite | Hueco | `TestBordes_CuerpoSobreElLimite` |
| Reintentos | Solo existen en `mailer.Retrying`, ya probado | Sin cambios |
