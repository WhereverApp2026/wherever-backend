# NV-11 — Datos gestionados externos (Redis y Neo4j)

## Contexto
Hoy la API, Redis y Neo4j corren como contenedores en la misma instancia EC2 (nv-07). La especificación del curso permite "un servicio externo para base de datos" y pide costo cero. Esta spec saca Redis y Neo4j de la instancia y los pasa a servicios gestionados externos, como MongoDB ya está en Atlas. Así la API queda sin datos locales y puede replicarse en NV-12. **Depende de**: NV-07 (Parameter Store, Terraform) y NV-08 (CD).

## Requisitos
1. **Neo4j** pasa a Neo4j AuraDB Free y **Redis** a Upstash Redis Free, creados con el correo único del proyecto.
2. Conexión siempre cifrada (`neo4j+s://` y `rediss://`) y autenticada. Las credenciales viven en Parameter Store (`/wherever/dev/`), nunca en Git, Terraform ni en la línea de comandos; `deploy-env.sh` arma el entorno sin imprimirlas.
3. El código de la API solo cambia en lo que este requisito y el 4 piden. `redis.New` y `neo4j.New` ya aceptan `rediss://` y `neo4j+s://`. Los errores de una URL mal formada no repiten la contraseña, porque acabarían en el log.
4. **Presupuesto de comandos de Redis.** Se mide cuántos comandos genera cada operación (`scripts/medir-redis.sh`) y se estima el mes con dos APIs. El límite de Upstash Free es de 500 000 comandos al mes; si la estimación pasa del 60 % (300 000), se reducen comandos o se cambia de plan o proveedor. El plan B es una instancia propia de Redis. Upstash cobra `MULTI` y `EXEC`, y no cobra `PING`, `HELLO` ni `AUTH`. Para entrar en el presupuesto, el limitador usa `Pipeline` en vez de `TxPipeline` y `/health` no pasa por él. `TestPresupuestoRedis_*` fija las cifras para que no suban sin rehacer la estimación.
5. **Neo4j Free** se pausa tras 72 horas sin uso y, si sigue en pausa 30 días, **se borra con sus datos**. Sus límites no son claros: la página del plan dice 50 000 nodos y 175 000 relaciones, y la FAQ dice 200 000 y 400 000; se usa la cifra menor hasta confirmarla en la consola. Se mide el tamaño del grafo. Se documenta cómo reanudarlo, y antes de la entrega y de la evaluación se verifica que no esté en pausa.
6. Sin migración de datos. Aún no hay modelo de grafo ni consultas en la API, así que esta spec define el mínimo: restricciones de `Usuario`, `Lugar` y `Categoria`, y `scripts/siembra` (idempotente) con datos de demostración. `/indexes` sigue siendo solo de MongoDB: correrlo contra Neo4j en cada despliegue dejaría el despliegue a merced de la pausa. Ambos scripts forman parte del "script de la base de datos" de la entrega.
7. Los contenedores locales de Redis y Neo4j se quitan de `server/compose.yml` solo después de verificar la API contra los servicios externos.
8. Si Neo4j no responde, `/ready` da 503 y la API no se cae. Si Redis no responde, `/ready` da 200 `degraded` (constitución §2.1): el limitador pasa a memoria, y revocación, bloqueo de login y candados responden 503. Los tiempos de espera están acotados.
9. Regiones lo más cerca posible de `us-east-2` entre las que ofrezca el plan gratuito, con latencia medida y registrada. Si los tiempos actuales (Redis 300 ms, `/ready` 1 s) no alcanzan, se ajustan con la medición en la mano.
10. `verificar.sh` comprueba que la instancia no corre ni escucha Redis o Neo4j, y que `/ready` da 200 con `estado: ok`.

## Diseño
- Parámetros, ambos SecureString: `REDIS_ADDR` (URL `rediss://` completa) y `NEO4J_URI` (`neo4j+s://` con usuario y contraseña en la URI). Son las claves que la API ya lee, así que no hacen falta claves nuevas (decisión de nv-01, `Memoria.md`). `deploy-env.sh` ya vuelca `/wherever/dev/` al entorno de la API y excluye el token de GHCR.
- `server/compose.yml` hoy fija `REDIS_ADDR` y `NEO4J_URI` en `environment`, lo que tapa a `.env`: esas líneas salen primero, con los contenedores aún presentes.
- `server/compose.yml` conserva solo `api` (y Caddy mientras NV-12 no exista).
- **Terraform:** el grupo de seguridad solo deja salir 443, 80 y 27017, así que se agregan las salidas 6379 (Upstash) y 7687 (Aura). Sin recursos de otro tipo.
- Los planes gratuitos no ofrecen lista de IP permitidas: la protección es TLS más credenciales fuertes y rotables.
- El esquema y la siembra se corren a mano con `make siembra` (`scripts/siembra`).
- Cambio de proveedor posible sin tocar código: solo cambian los parámetros.

## Contratos
Sin endpoints nuevos ni cambios de contrato. `/health` deja de contar en el límite global (`docs/endpoints.md`). Enmienda de la constitución: Redis y Neo4j gestionados externos en desarrollo, y `/health` fuera del limitador.

## Tests
- `go build ./...`, `go vet ./...` y las cinco suites sin cambios.
- Medición de comandos de Redis por operación y estimación mensual, registradas en `Memoria.md`.
- Tamaño del grafo frente al límite de Aura Free, registrado.
- Unit: `TestNew_URLCifradaActivaTLSYCredenciales`, `TestSplitAuth_*`, `TestNew_ElErrorNoRepiteLaClave`.
- Integration: `TestPresupuestoRedis_*`. Database: `TestGrafo_SiembraIdempotente`.
- Flujo de extremo a extremo (registro, inicio de sesión, refresco, cierre de sesión) contra los servicios externos. La API no tiene endpoints de grafo: la lectura la hace `make siembra`, que cuenta nodos y relaciones, y `/ready`.
- Con Neo4j inalcanzable o credenciales erróneas, `/ready` da 503; con Redis inalcanzable, 200 `degraded`. En ambos casos la API sigue viva.
- `make siembra` idempotente contra el Neo4j externo.
- `verificar.sh` exit 0; `terraform plan` sin cambios o sin reemplazos.

## Criterios
- Sin credenciales en Git, Terraform ni logs (`critic B1 → PASS`).
- Redis y Neo4j fuera de la instancia de la API.
- Estimación de consumo de Redis por debajo del 60 % del límite gratuito, o plan B ejecutado.
- Costo adicional cero.

## Límites
- Planes gratuitos sin SLA, sin copias de seguridad (Aura Free) y con cuotas. Agotar la cuota de Redis deja en 503 todo lo autenticado y el login; la pausa de Neo4j deja `/ready` en 503 aunque ninguna ruta use el grafo, y eso frena al actualizador de nv-08.
- **Un solo cliente puede agotar la cuota:** el límite global deja 1000 peticiones por minuto a cada IP, o sea 3000 comandos por minuto. Los 500 000 se acaban en menos de 3 horas. El plan gratuito no lo evita; si pasa, se aplica el plan B. Upstash no documenta qué hace al llegar al límite en este plan.
- **NV-12** debe usar `/health` en el chequeo del balanceador, no `/ready`: con su intervalo de 15 s serían 887 000 comandos al mes, el 177 % de la cuota, y una pausa de Neo4j sacaría a las dos instancias a la vez.
- Dependencia de proveedores externos y latencia mayor que en red local.
- Sin lista de IP permitidas en esos planes.
- Datos de desarrollo únicamente.

## Aprobación y Control de Cambios
- **Estado:** 🚧 Implementada en la rama `nv-11-datos-externos`; sin firmar
- **Revisores:** Osmar Reséndiz (pendiente de aprobar)
- **Fecha:** 2026-11-05 (inicio de la ejecución)
- **Decisiones para el revisor antes de ejecutar:** (a) Upstash Redis Free y Neo4j AuraDB Free; (b) umbral del 60 % y plan B con instancia propia de Redis; (c) cuentas con el correo único del proyecto; (d) enmienda de la constitución.
- **Detalle:** la evidencia irá en `specs/nv-11/checklists/gates.md` + `Memoria.md`.
