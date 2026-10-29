# Endpoints — vivo (nv-01)

> Fuente de contratos. Las specs solo enlazan, no duplican. Constitución §6.
> Las filas de módulos de negocio se agregan con su spec (`nv-04+`), copiadas de `Wherever_Endpoints_CRUD.xlsx` (hojas A1–A7, J1–J9).

| Método | Ruta | Auth | Rate | Códigos | Spec |
|---|---|---|---|---|---|
| GET | `/health` → `{estado:"ok", version}` | no | global | 200 | nv-01 |
| GET | `/ready` → `{estado, dependencias}` (MongoDB, Redis, Neo4j; 1 s c/u; Redis caído = 200 `degraded`) | no, red interna | global | 200/503 | nv-01 |
| GET | `/api/v1/ping` | no | global | 200 | nv-01 |
| GET | `/api/v1/probe/timeout` (no-prod) | no | global | 408/404 | nv-01 |
| GET | `/api/v1/probe/error` (no-prod) | no | global | 500/404 | nv-01 |
| GET | `/api/v1/probe/forbidden` (no-prod) | no | global | 403/404 | nv-01 |
| GET | `/api/v1/probe/conflict` (no-prod) | no | global | 409/404 | nv-01 |
| GET | `/api/v1/probe/invalid` (no-prod) | no | global | 422/404 | nv-01 |
| GET | `/docs` y `/openapi.json` (solo dev) | no | — | 200/404 | nv-01 |

Convención JSON: `snake_case`. `PageResult{items, siguiente_cursor, limite}` (desde nv-02). Errores `{codigo, mensaje, campos[], detalle?, traza_id}`.

## Diferidos (sin ruta en código todavía)
- `GET /metrics` (Prometheus, red interna) — spec de observabilidad. Hoja J9.
- `POST /api/v1/errores` y `GET /api/v1/admin/errores` (colección `errores_app`) — spec de observabilidad. Hoja J9.
