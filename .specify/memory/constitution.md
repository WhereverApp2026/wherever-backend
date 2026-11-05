---
version: 1.1.0
ratified: 2026-10-28
last_amended: 2026-11-04
status: borrador (pendiente de firma del revisor)
source_of_truth: .specify/memory/constitution.md
mirror: ./constitution.md
methodology: SDD
---

# Constitución — Wherever Backend

> Rector SDD. Ante conflicto con otro artefacto, **manda esta constitución**.
> Filosofía madre: cada módulo sale liberable con todo `00+01+02+03+04` aplicado.
> Fuera de alcance en esta etapa (`nv-00…nv-03`): módulos de negocio (usuarios, planes, grupos, reservas, pagos), despliegue en AWS/Terraform, CD, caos, rendimiento y mutación obligatorios.

## 1. Propósito y alcance

- API REST en Go + Fiber, **stateless y multi-instancia (mínimo 2)**, con MongoDB, Neo4j y Redis.
- Metodología SDD. Toda funcionalidad nace de una spec en `/specs` (`nv-00…nv-0X`).
- Objetivo de esta etapa: dejar la **arquitectura base** (stack, pipeline, salud, molde de módulo y puerta de calidad) sin ninguna funcionalidad del producto.
- Contrato de producto: `Wherever_Endpoints_CRUD.xlsx` (Equipo IntegradOps, 237 endpoints en las hojas A1–A7 y J1–J9) y los documentos de diseño de la app y del backend. Cada spec `nv-04+` lleva sus filas a `docs/endpoints.md`; si choca con esta constitución, se enmienda una de las dos, nunca se ignora.

## 2. Principios no negociables

1. **Stateless multi-instancia**: sin sesión en el proceso. El estado vive en MongoDB o Redis. Si Redis cae, `/ready` lo reporta como `degraded` (no es verde falso) y:
   - el rate limit usa un respaldo en memoria **por instancia** con aviso en el log (el límite efectivo pasa a ser N × límite; se acepta por ser temporal);
   - lo que protege al usuario o al dinero es **fail-closed**: revocados, bloqueo de login, idempotencia y candados responden `503` en vez de suponer un valor. Nunca se guardan en memoria del proceso.
2. **Seguridad primero**: validar en la frontera, denegar por defecto, mínimo privilegio, sin secretos en código, logs ni errores.
3. **Sin estado oculto**: todo se comparte por constructores (inyección explícita); lo global expone `Reset()` y usa `sync.Mutex`.
4. **Fail-fast**: errores uniformes, `/ready` con checks reales de las tres bases, sin tragar errores. `detalle` solo en no-prod.
5. **Empírico antes que teórico**: medir antes de fijar timeouts, nombres de métricas o umbrales.
6. **Documentar límites** en `Memoria.md`; prohibido el workaround sin issue.
7. **Real-first**: prohibido hash falso, token opaco o repositorio simulado en integración. Cada módulo usa `JWT real + Argon2id real` desde el primero.

## 3. Stack obligatorio

| Capa | Tecnología |
|---|---|
| Runtime | Go 1.22+ / Fiber v2 |
| Persistencia | MongoDB (driver oficial), Neo4j (driver oficial), Redis (`go-redis`). Nunca una base simulada en integración |
| Tests | `testing` + `testify` + `mockery` (solo interfaces externas); Testcontainers-go con puerto fijo |
| Auth | JWT HS256 (`golang-jwt`), access 15 min + refresh 30 días con rotación y revocación por familia; Argon2id 64MB/3iter (`x/crypto/argon2`). Roles: `consumidor`, `comerciante`, `administrador` |
| Rate | Global 1000/min, Login 5/5min, Admin 200/min, Writes 10 concurrentes por usuario (Redis) |
| Validación | `go-playground/validator` |
| Logs | `slog` JSON + `request_id` |
| SAST | Semgrep + `gosec` + `golangci-lint` + `govulncheck` |
| CI | GitHub Actions: `ci-pr.yml` sin deploy; `cd-dev.yml` publica la imagen y el entorno de desarrollo se actualiza solo (nv-08); `ci-pr.yml` incluye análisis estático, de imagen y gate, y `nightly.yml` incluye DAST (nv-09); producción sin CD |
| Infra de desarrollo | Docker Compose (`deployments/docker-compose.yml`: MongoDB, Redis, Neo4j) con `make infra-up`/`infra-down`; `Dockerfile` multi-etapa en la raíz |
| Infra (posterior) | Docker, Terraform, AWS (ALB con SSL, ≥2 instancias en zonas distintas, S3 + CDN, Secrets Manager) |

No se agregan librerías sin evaluar licencia, mantenimiento y seguridad. Regla de excepción en `nv-00`.

## 4. Arquitectura

- Capas `handler → service → repository`; el handler no toca la base y el repository no conoce HTTP. Módulos por funcionalidad en `internal/<modulo>/`.
- Rutas `/api/v1/<recurso>`, v1 estricta; sin versión → 404.
- JSON `snake_case`. Errores `{ codigo, mensaje, campos[], detalle?, traza_id }`; `traza_id` = `request_id` (cabecera `X-Request-ID`).
- Pipeline fijo: `SecurityHeaders outermost → Recover/ErrorHandler → RequestID → ProxyHeaders → CORS 1 origen → RateLimiter antes de Auth → JWT → Roles → Handlers`.
- Health: `/health` (liveness, siempre 200 `{estado:"ok", version}`, sin consultar bases) y `/ready` (check real de MongoDB, Redis y Neo4j con 1 s máximo cada una; responde `{estado, dependencias}`; 503 si cae MongoDB o Neo4j; Redis caído = `degraded` con 200). Ambos fuera de `/api/v1`; `/ready` es para la red interna y el balanceador.
- OpenAPI solo en desarrollo.
- DTOs validados; códigos `200/201/202/204/400/401/403/404/408/409/410/422/423/429/500/503`. `400` = petición mal formada, `422` = falla de validación o regla de negocio, `410` = vencido, `423` = bloqueado. Paginación por cursor `?cursor=&limite=` → `PageResult{items, siguiente_cursor, limite}` (por defecto 20, máx. 50; `siguiente_cursor` vacío = no hay más).
- Versionado: un cambio que rompa la app crea `/api/v2` sin retirar `/api/v1`.

## 5. Código y API

- Inglés en código y carpetas; comentarios, mensajes y documentación en español.
- Colecciones en español, minúsculas y plural; campos `snake_case`; `_id` ObjectId; `creado_en` y `actualizado_en` en UTC; dinero en centavos (entero).
- Concurrencia optimista con campo `version` (entero) y cabecera `If-Match`.
- Idempotencia: las operaciones de dinero o que la app reintenta (reservas, pagos, gastos, mensajes) aceptan `Idempotency-Key` o `id_cliente`.
- Borrado lógico con `eliminado_en`; el borrado definitivo lo hacen trabajos programados o índices TTL.
- Fechas en UTC (ISO 8601). Los horarios de negocio se interpretan en la zona IANA de su ciudad (por defecto `America/Mexico_City`), porque la app se usará en varios países.
- Texto para buscar en campos `*_normalizado` (minúsculas, sin acentos).
- Índices con prefijo por tipo: `ux_` único, `ix_` normal o compuesto, `ttl_` TTL, `geo_` geoespacial, `txt_` texto.
- Complejidad `gocyclo ≤ 10` por función. `golangci-lint` con `gosec` y `errcheck` como error.
- Sin `recover` ad-hoc en handlers. Errores tipados en `internal/shared/apperr`: `BadRequest→400, Unauthorized→401, Forbidden→403, NotFound→404, Timeout→408, Conflict→409, Gone→410, Validation→422, Locked→423, RateLimited→429, Unavailable→503`, cualquier otro → 500.
- Índices de MongoDB solo por script idempotente (`scripts/indexes`), con nombres sin fechas.

## 6. Seguridad DevSecOps

- Headers: `nosniff, DENY, strict-origin-when-cross-origin, X-XSS-Protection 0, CSP default-src 'none', HSTS 365d solo no-dev, CORS de 1 origen`. Límite de cuerpo 10MB y timeout 60s.
- JWT: `clave ≥ 32B solo por env/Secrets Manager, leeway 0, métodos válidos [HS256], iss/aud, claims sub/jti/roles/sid, revocación jwt_revocado:{jti} en Redis`. Un token `alg:none` o sin firma siempre da 401. El refresh se guarda solo como hash SHA-256 (es aleatorio de alta entropía, no una contraseña).
- Login: hash falso para respuesta idéntica (anti-enumeración) y bloqueo `5 intentos → 15 min` en `login_fallos:{correo}`, fail-closed. El contador corre aunque el correo no exista, así el `423` con minutos restantes no revela cuentas.
- Llaves de Redis (todas con TTL): `jwt_revocado:{jti}`, `login_fallos:{correo}`, `rate:{politica}:{sujeto}`, `idem:{llave}`, `lock:{recurso}`, `cuota:{usuario}:{fecha}`, `cache:{entidad}:{id}` (esta última desde su spec de módulo).
- La contraseña nunca va a caché, logs ni respuestas.
- Logs sin secretos (filtro en `slog`); `request_id` por petición.
- Imágenes y dependencias: Dockle y Trivy en cada PR, ZAP pasivo cada noche (nv-09). RESTler y Cosign se documentan como deuda diferida, no exigidas en esta etapa.
- `docs/endpoints.md`, `docs/rate-limit-matrix.md` y `docs/asvs-l2-checklist.md` están vivos; las specs solo los enlazan.

## 7. Testing y QA

- Suites: `Unit`, `Integration` (bases reales con Testcontainers), `Security`, `Database` (índices), `Contract` (captura `CONTRACT_CAPTURE=1`). Mutación, rendimiento y caos son nightly u opcionales, no bloquean.
- Cobertura `≥ 45 %` con `scripts/check_coverage.sh --min 45` (único bloqueo de cobertura hoy).
- Puerta incremental advisory (`nv-03`): `scripts/check_delta_coverage.py --min 80 --advisory` + mutación enfocada en lo cambiado (`continue-on-error`).
- Contenedores de prueba con puerto fijo; suites con contenedor nunca en paralelo local.

## 8. CI y release manual

- CI-PR en jobs paralelos (nv-09): `changes`, `guardrails` (critic → endpoints → sdd), `semgrep`, `go-security` (golangci-lint → gosec → govulncheck), `image-scan` (build → Dockle → Trivy), `pipeline` (`deps → build → unit → integration → security → database → contract → coverage → delta-check → mutation`, siempre con `-count=1`) y `pr-gate`.
- Producción sin CD; el entorno de desarrollo se actualiza solo (nv-08). Release manual: `go build` + imagen Docker + variables `APP_ENV, MONGO_URI, REDIS_ADDR, NEO4J_URI, JWT_KEY, JWT_ISSUER, JWT_AUDIENCE` + `curl /ready + login + 401 + headers` + tag `vX.Y.Z-(modulo)N`.
- `CHECKLIST_PR.md` obligatorio. 1 revisor. `main` se protege por acuerdo: el plan actual de GitHub no permite exigirlo (riesgo aceptado solo para desarrollo, nv-08).
- YAML validado con `yaml.safe_load`.

## 9. Observabilidad mínima

- `slog` JSON + `request_id` + `/health` + `/ready`. Sin OTel/Prometheus/Grafana completos por ahora (spec de observabilidad posterior).
- Auditoría con `traza_id`, sin datos personales en logs.
- Diferido a la spec de observabilidad (lo piden los documentos de diseño): `/metrics` para Prometheus, Loki, alertas en Grafana y `POST /api/v1/errores` (hoja J9).

## 10. Memoria

- `Memoria.md` por fase o slice: estado, decisiones con su porqué y errores a evitar. Máximo 10000 líneas. Sin datos sensibles.

## 11. Gobernanza SDD

- Specs en `/specs/nv-0X` con plantilla de 7 secciones + Aprobación. Sin cambio de contrato sin spec. `AGENTS.md` dice cómo; la constitución dice qué no se negocia.

## 12. Control de cambios

| Versión | Fecha | Cambio |
|---|---|---|
| 1.0.0 | 2026-10-28 | Versión inicial |
| 1.1.0 | 2026-10-29 | Alineada con `Wherever_Endpoints_CRUD.xlsx` y los documentos de diseño: roles, vida de los tokens, llaves de Redis, códigos 410/423/503, `traza_id`, idempotencia, borrado lógico, zonas horarias, nombres de índices, Docker Compose de desarrollo y regla fail-closed con Redis caído |
