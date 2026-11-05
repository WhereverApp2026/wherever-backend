# gates.md — NV-11 Datos gestionados externos

| Gate | Criterio | Evidencia | Estado |
|---|---|---|---|
| critic B1 | Sin credenciales en Git, Terraform ni estado | `critic-guardrails: PASS`; parámetros solo en Parameter Store | listo |
| critic B2 | Sin credenciales en logs | 0 líneas con la clave en los logs de las pruebas; `TestNew_ElErrorNoRepiteLaClave` | listo |
| critic B3 | Pipeline sin cambios | `critic-guardrails: PASS` | listo |
| critic B4 | Endpoints sin cambios | `check_endpoints: OK (37 rutas)`; solo cambia el límite de `/health` | listo |
| Comandos Redis | Estimación mensual bajo el 60 % del límite, o plan B | `Memoria.md`, `TestPresupuestoRedis_*` | 40 % estimado, falta contrastar con la consola de Upstash |
| Grafo | Tamaño bajo los límites de Aura Free | `make siembra` contra Aura: 9 nodos y 9 relaciones, dos corridas | listo |
| Flujo | Registro, inicio de sesión, refresco, cierre y lectura del grafo | Servidor público: 201, 200, 200, 204 y token revocado 401; grafo leído por `make siembra` | listo |
| Fallo | `/ready` 503 con Neo4j caído; 200 `degraded` con Redis caído; la API sigue viva | Prueba en el servidor con un contenedor aparte: clave errónea e inalcanzable; `/health` 200 | listo |
| Instancia | Sin 6379 ni 7687 escuchando | `ss`: 22, 80, 443, 8080 y locales; `verificar.sh` | listo |
| Scripts | Esquema y siembra incluidos para la entrega | `scripts/siembra`, `TestGrafo_SiembraIdempotente`, corrida contra Aura | listo |
| `verificar.sh` | exit 0 | `verificar: OK`; `terraform plan`: sin cambios | listo |
