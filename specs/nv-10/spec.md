# NV-10 — Fuzzing nativo y pruebas de integración de bordes

## Contexto
Las suites actuales cubren el camino feliz y casos de error conocidos. Falta probar entradas hostiles y fallos de dependencias. Esta spec agrega fuzzing nativo de Go y pruebas de bordes, sin herramientas externas. **Depende de**: nada técnico; se recomienda después de NV-09 para que el gate ya las cuente.

## Requisitos
1. **Fuzz nativo (`go test -fuzz`)** sobre el código propio que procesa entradas: parsers de payload, DTOs y validaciones de cada módulo. El inventario inicial de objetivos se arma en `plan.md` recorriendo los módulos existentes.
2. **Seguridad:** fuzz de la validación de JWT (tokens malformados, `alg: none`, firma alterada, claims inesperados) y del análisis y verificación del hash codificado de Argon2id. Invariantes: nunca hay pánico, nunca se acepta un token o hash inválido, y un token aceptado lleva una firma HMAC correcta comprobada por aparte. No se fuzzea el cálculo de Argon2id; en pruebas se usan parámetros bajos.
3. **Handlers de Fiber:** construir la app real **con sus dependencias caídas** y llamar `app.Test()` con peticiones generadas (ruta, parámetro, consulta, cabeceras, cuerpo). Invariantes: sin pánico, sin 5xx salvo el 503 de las bases caídas, y los errores respetan el formato de error del contrato. Las rutas salen del código; `docs/endpoints.md` es solo referencia.
4. **Casos semilla** con `f.Add`, y en `testdata/fuzz/` los que encuentre el fuzz, versionados. En PR, `go test` los corre como pruebas normales. El fuzzing real corre en `nightly.yml` con `-fuzztime` acotado (por ejemplo 60 s por objetivo, máximo unos 15 min en total). Todo fallo encontrado se guarda como caso semilla y se corrige en el mismo PR.
5. **Integración de bordes:** primero se revisa qué cubren las cinco suites y se listan los huecos. Se usa el mecanismo que ya existe para levantar Mongo, Redis y Neo4j; Testcontainers-go solo si no hay otro. Escenarios: dependencia caída o lenta (`/ready` 503, la API sin pánico y con error controlado), payloads malformados de extremo a extremo y tiempos de espera. Los reintentos se prueban solo donde el código ya los implementa.
6. La cobertura no baja del gate actual (45 %).

## Diseño
- Archivos `*_fuzz_test.go` junto a cada paquete, con `f.Add` de semillas realistas.
- **Sin repositorios simulados:** handlers y services usan structs concretos sobre Mongo, sin interfaces, y `AGENTS.md` §3 y §7 limitan los mocks a interfaces externas. Simularlos obligaría a refactorizar el backend. `internal/testutil/authtest` arma la app real contra `testutil.DeadAddr` y `DeadRedis`: lo inválido se rechaza antes de tocar la base y lo válido recibe 503. No usa red externa ni Docker.
- **Rutas con sesión:** en el fuzz no pasan de la revisión en Redis. Las cubre `TestBordes_PayloadsHostilesEnTodasLasRutas` con bases reales y una tabla de cuerpos hostiles.
- **Dependencia lenta:** un socket que acepta y nunca responde, igual que `slowSMTP`; no hace falta pausar contenedores.
- **Hash de Argon2id:** `parseHash` valida formato y topes antes de calcular. Con `t=0` o `p=0` `argon2.IDKey` entraba en pánico, y con una memoria enorme el proceso moría sin poder atraparlo.
- Pruebas de integración de bordes en `tests/integration/bordes_test.go`.
- `nightly.yml`: job `fuzz`. `scripts/ci/fuzz.sh` busca los objetivos en el código, los recorre con `-fuzztime` y sube logs y fallos como artefacto. La entrada `solo` (`todo`, `dast`, `fuzz`) reemplaza a `solo_dast` de nv-09.

## Contratos
Sin endpoints nuevos ni cambios de contrato. Si el fuzzing revela un 5xx por entrada inválida, se corrige para que devuelva el error de contrato y se documenta en `docs/endpoints.md` (critic B4).

## Tests
- `go build ./...`, `go vet ./...` y las cinco suites sin cambios.
- Cada objetivo de fuzz corre unos segundos en local sin fallos y sus semillas pasan con `go test`.
- **Prueba de que detecta:** un error plantado en una rama de prueba (por ejemplo una validación que entra en pánico con un campo vacío) debe ser encontrado por el fuzz o por las semillas.
- Una corrida manual de `nightly.yml` con el job `fuzz` dentro del tiempo previsto.

## Criterios
- Existen objetivos de fuzz para parsers, DTOs, validaciones, JWT, hash y handlers de mayor riesgo, listados en `plan.md`.
- Ningún fallo conocido queda sin semilla ni corrección.
- El PR no se alarga de forma apreciable; lo lento corre de noche.
- Las pruebas de bordes cubren al menos Mongo, Redis y Neo4j caídos.

## Límites
- Sin Pact (se difiere por costo y complejidad). La mutación ya existente (nv-03) no cambia.
- El fuzz no ejercita la lógica detrás de una base real: solo llega hasta el repositorio. Esa lógica sigue en las suites.
- Con `APP_ENV=staging`, `detalle` trae el texto del driver a propósito (constitución §2): ante una caída de Atlas, el servidor público mostraría nombres de host. No se cambia aquí; queda como decisión aparte.
- Un cuerpo sobre el límite responde 413 con `traza_id` vacío, porque el rechazo ocurre antes del middleware.
- Sin RESTler ni fuzzing a nivel de red contra el servidor desplegado.
- El fuzzing no sustituye una revisión de seguridad.

## Aprobación y Control de Cambios
- **Estado:** ✅ Aprobada con pendientes (fuzz nocturno y pruebas de bordes activos)
- **Revisores:** Osmar Reséndiz
- **Fecha:** 2026-11-05 (aprobación)
- **Correcciones al ejecutar (2026-11-05):** no hay repositorios simulados y la spec contradecía `AGENTS.md`; `critic B3` no revisa `nightly.yml`; la mutación ya existía; las caídas ya estaban cubiertas en auth y `/ready`, y faltaban lo lento y los demás módulos. **Hallazgos corregidos:** el pánico y el agotamiento de memoria al verificar un hash corrupto, y `NormalizePhone`, que dejaba saltos de línea entre separadores (encontrado por el fuzz).
- **Decisiones del revisor, dadas al ordenar ejecutarla el 2026-11-05 (confirmar al firmar):** (a) usar el mecanismo de contenedores que ya tienen las suites y Testcontainers-go solo donde falte; (b) tiempo nocturno máximo de fuzzing (propuesta: 15 min); (c) Pact fuera y la mutación sin cambios.
- **Detalle:** la evidencia irá en `specs/nv-10/checklists/gates.md` + `Memoria.md`.
- **Pendientes aceptados al firmar (2026-11-05):** `FuzzHandlers` da pocas ejecuciones en Actions (unas 6 000 por noche); `detalle` visible en staging, por decidir aparte.
