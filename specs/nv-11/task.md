# task.md — NV-11 Datos gestionados externos

## Fase 0 — Preparación
- [x] T01 Decisiones (a)–(d) aceptadas por Osmar Reséndiz al pedir la ejecución; firma pendiente.
- [x] T02 Rama `nv-11-datos-externos`.

## Fase 1 — Cuentas y parámetros (Req. 1, 2)
- [x] T03 Cuentas en Upstash y Neo4j Aura con el correo del proyecto; regiones cercanas.
- [x] T04 Operador carga `REDIS_ADDR` y `NEO4J_URI` en Parameter Store.

## Fase 2 — Mediciones (Req. 4, 5, 9)
- [x] T05 Medir comandos de Redis por operación y estimar el mes.
- [x] T06 Decidir: seguir o plan B si pasa del 60 %. Se reducen comandos y se sigue.
- [x] T07 Medir tamaño del grafo y latencia; registrar.

## Fase 3 — Esquema y datos (Req. 6)
- [x] T08 `make siembra` contra Neo4j externo.
- [x] T09 Script de siembra de demostración (probado contra Neo4j local).

## Fase 4 — Cambio de la API (Req. 3, 7, 8, 10)
- [x] T10 Probar la API contra los servicios externos en local.
- [x] T11 `terraform apply` con las salidas 6379 y 7687; subir `compose.yml` sin las líneas que tapan a `.env`; instalar la API contra los servicios externos.
- [x] T12 Flujo de extremo a extremo.
- [x] T13 Prueba de fallo (credenciales erróneas, servicio inalcanzable) y `/ready` 503.
- [x] T14 `verificar.sh` actualizado y en exit 0.

## Fase 5 — Cierre
- [x] T15 Quitar Redis y Neo4j de `compose.yml` y retirar contenedores y volúmenes tras verificar.
- [x] T16 Procedimiento para reanudar Neo4j y para rotar credenciales en `README.md`.
- [x] T17 Enmienda de la constitución, `AGENTS.md`, `CHECKLIST_PR.md`, `Memoria.md`.
- [ ] T18 `go build`, `go vet`, cinco suites, `check_sdd`, `critic`; PR; evidencia en `gates.md`.
