# plan.md — NV-75 Consejo del día y descripción con IA para el negocio

1. Mapeo
- Crear: `internal/consejos/{model,repository,reglas,service,handler,dto}.go`, `internal/negocios/panel/descripcion.go` (service, DTO y ruta), `internal/testutil/consejostest` y pruebas de las cinco suites.
- Modificar: `internal/platform/config` y `configs/.env.example` (`IA_CONSEJO_DIA`), `internal/indexes` + `scripts/indexes` (`ux_consejo_dia`, TTL), `cmd/api` (guardia, fuentes y observador de baja), `scripts/routes`, `docs/endpoints.md`.
- Sin cambios: `internal/ia` (se usa como está), políticas de Redis, límites `IA_LIMITE_*`.
- Requiere nv-74 en `main` (lee `estadisticas_diarias`); si no está, la regla de franja floja queda fuera hasta que llegue.

2. Guardarraíles
- Servicios reales en las pruebas; `ia.Modelo` de prueba solo como interfaz externa; al modelo solo agregados; logs sin contexto ni salida.
- `TestPresupuestoRedis_ConsejoNV75` fija el costo.

3. Secuencia
1. Preparación: colección, índices y variable.
2. Implementación: reglas y plantillas; consejo con candado por índice y cupo diario; descripción con respaldo.
3. Cierre: pruebas, documentos y `Memoria.md`.

4. Verificación
- `go build`, `go vet`, Unit y lint en local; Integration, Security, Database y Contract en el CI del PR; `critic`, `check_endpoints`, `check_sdd`, cobertura ≥ 45 %, Semgrep en 0.
