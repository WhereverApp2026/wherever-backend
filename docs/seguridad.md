# Seguridad — estado del proyecto y brecha frente a banca y fintech

> Fecha de corte: 2026-11-19 (`main` en `d6575c6`, PR #125). Documento para la evaluación del 2 de diciembre de 2026.
> Vivo: se actualiza cuando cambia un control. La constitución (`.specify/memory/constitution.md` §6) manda ante conflicto.
> Detalle por requisito ASVS: [`asvs-l2-checklist.md`](asvs-l2-checklist.md). Límites de peticiones: [`rate-limit-matrix.md`](rate-limit-matrix.md). Operación en AWS: [`infra/aws-dev/README.md`](../infra/aws-dev/README.md).

## 1. Resumen

Wherever es una API REST en Go (Fiber), monolito modular, con MongoDB Atlas, Neo4j Aura y Redis en Upstash, desplegada en AWS (`us-east-2`) con dos EC2 detrás de un balanceador y CloudFront. La seguridad se diseñó desde la constitución y se prueba en cada PR: autenticación robusta (Argon2id, JWT de 15 min con revocación), autorización por dueño del recurso, cifrado de campos sensibles, análisis estático y de dependencias bloqueantes, escaneo de imagen y de Terraform, imagen firmada y verificada antes de instalarse, y respaldos semanales con alarma. Hay unas 660 pruebas unitarias, 461 de integración, más de 105 de seguridad, 9 de base de datos y 21 de contrato, además de fuzzing, mutación y ZAP pasivo cada semana.

Lo que falta para un nivel bancario no está en el código sino alrededor: el gobierno del repositorio en GitHub (protección de `main`, alertas), un entorno de staging separado, TLS completo hasta el balanceador (requiere dominio propio), pruebas de penetración externas y procesos formales (incidentes, rotación, auditoría inmutable).

### Puntaje DevSecOps (rúbrica de 20 controles)

1 = completo con evidencia en el repo · 0,5 = parcial · 0 = falta. Lo que depende de la configuración de GitHub solo cuenta si se comprobó activo.

| # | Control | Punt. | Evidencia |
|---|---|---|---|
| 1 | SAST bloqueante | 1 | Semgrep (`p/golang`, `p/owasp-top-ten`, `p/secrets` y `.semgrep/semgrep.yaml`), gosec y golangci-lint en `ci-pr.yml` (jobs `semgrep`, `go-security`) |
| 2 | Dependencias vulnerables (SCA) | 1 | `govulncheck` bloqueante en `go-security`; Trivy sobre la imagen; `.github/dependabot.yml` semanal para `gomod`, `github-actions` y `docker` (#120) |
| 3 | Secretos en el historial | 1 | gitleaks v8.30.0 en `guardrails` sobre los commits del PR; `.gitleaks.toml` con excepciones por valor exacto (#120) |
| 4 | Escaneo de imagen | 1 | Dockle y Trivy bloqueantes en `image-scan`, fijados por digest |
| 5 | Escaneo de IaC | 1 | `trivy config` de `infra/aws-dev` en `iac-scan`; HIGH/CRITICAL bloquean; excepciones `#trivy:ignore` con razón (#120) |
| 6 | DAST | 0,5 | ZAP **pasivo** semanal en `nightly.yml` con `.zap/plan.yaml` (#118). Falta escaneo activo contra un staging |
| 7 | Fuzzing | 1 | Objetivos `Fuzz*` como pruebas con semillas y fuzz real semanal con `scripts/ci/fuzz.sh`; casos en `testdata/fuzz/` (#118) |
| 8 | Pruebas de seguridad y mutación | 1 | `tests/security/` (401, 403, aislamiento entre personas, 429) incluida `rutas_faltantes_security_test.go` (#119); mutación de PR y nocturna con `scripts/mutation.sh` |
| 9 | Autenticación y sesión | 1 | Argon2id 64 MB/3; JWT HS256 15 min, refresh rotado y revocación `jwt_revocado:{jti}` fail-closed; bloqueo 5 → 15 min (`internal/auth/tokens.go`, `tests/security/auth_security_test.go`) |
| 10 | Control de acceso | 1 | Denegar por defecto, roles y dueño validado en el service; `/yo` con el `sub` firmado (`internal/http/middleware/roles.go`, `tests/security/*`) |
| 11 | Validación y manejo de errores | 1 | Validador en la frontera con registro único; error uniforme sin `detalle` en prod; cursores firmados (`internal/shared/pagination/cursor.go`) |
| 12 | Endurecimiento HTTP | 1 | Cabeceras, CSP, HSTS 365 d, CORS de 1 origen, 4 políticas de rate limit con 429 uniforme, body 10 MB, timeout 60 s (`internal/http/middleware/`) |
| 13 | Protección de datos | 0,5 | AES de campos sensibles (`internal/platform/cifrado`, alergias, fecha de nacimiento, CLABE en `internal/negocios/expediente`), teléfono como HMAC, EBS cifrado, buckets SSE-S3. **Falta:** el tramo CloudFront → balanceador → EC2 va en HTTP (`cdn.tf`: `origin_protocol_policy = "http-only"`) |
| 14 | Gestión de secretos | 0,5 | SSM Parameter Store SecureString; la imagen no lleva secretos; Atlas con usuario propio del servidor. **Falta:** rotación periódica (hoy manual, `infra/aws-dev/README.md` «Rotar credenciales») |
| 15 | Integridad de la cadena de suministro | 1 | `cd-dev.yml`: SBOM y provenance de buildx, firma cosign sin llaves del digest del índice; `actualizar.sh` exige `cosign verify` con la identidad `cd-dev.yml@refs/heads/main` antes de instalar (#122) |
| 16 | Infraestructura con mínimo privilegio | 1 | Deployer IAM con MFA, boundary (`infra/aws-dev/iam/`), sin claves de AWS en GitHub; IMDSv2 obligatorio, SSH cerrado (Systems Manager), 8080 solo desde el balanceador y este solo desde CloudFront (`alb.tf`) |
| 17 | Perímetro (WAF) | 1 | `infra/aws-dev/waf.tf` (PR E): web ACL en CloudFront con reputación de IP, límite por IP (429 uniforme), entradas maliciosas y OWASP general; el balanceador solo reenvía con la cabecera secreta de CloudFront, así nadie se salta el WAF. Cuenta desde que el operador lo aplica con `crear.sh` |
| 18 | Registro y monitoreo de seguridad | 1 | `slog` JSON con `request_id` y filtro de secretos y correos; CloudWatch con alarmas de errores, 5xx, dependencias y respaldos, y de ataque: picos de 401, 423 y 429 (`observabilidad.tf`). CloudTrail multirregión con validación de archivos, 365 días en S3, y alarmas por cambios de IAM y uso de root (`auditoria.tf`). Logs a 90 días. Cuenta desde que el operador lo aplica con `crear.sh` |
| 19 | Respaldo y recuperación | 0,5 | Respaldo semanal de MongoDB a S3 versionado, 28 días, la instancia solo escribe, alarma de 7 días y restauración documentada (#123, #124); `crear.sh` ya no oculta fallos (#125). **Falta:** copia fuera de la cuenta y prueba de restauración periódica; `destruir.sh` borra los respaldos |
| 20 | Gobierno del repositorio en GitHub | 1 | Ruleset «Protect main» activo: PR obligatorio, `pr-gate` obligatorio, sin force-push ni borrado, sin excepciones. Alertas de Dependabot activas. Secret scanning y push protection activados por el operador (ver abajo) |
| | **Total** | **18 / 20 = 90 %** | |

**Estado real de GitHub, consultado con `gh api` el 2026-11-19 después de que el operador lo activó** (la cuenta del agente tiene permiso `write`, no `admin`):

| Ajuste | Resultado | Cuenta |
|---|---|---|
| Protección de `main` | `rulesets` → «Protect main», `active`, sobre `refs/heads/main`, sin `bypass_actors`. `rules/branches/main` → `pull_request` (0 aprobaciones: un solo integrante de backend), `required_status_checks` = `pr-gate` (GitHub Actions), `non_fast_forward` y `deletion`. `branches/main` → `protected: true` | Sí |
| Alertas de Dependabot | `dependabot/alerts` → 200 con 0 alertas abiertas (antes 403 *«disabled»*). Actualizaciones de seguridad activadas por el operador; su endpoint solo lo lee un administrador | Sí |
| Secret scanning y push protection | Activados por el operador y la alerta de la llave de prueba cerrada como «Used in tests». No verificable con permiso `write` (`secret-scanning/alerts` → 404, `security_and_analysis` → `null`); el repositorio es público, donde ambos son gratuitos | Sí (reporte del operador) |

**Cómo leer el número.** La auditoría del 2026-11-18 daba ≈75 % (15/20); su desglose no quedó en el repositorio, así que esta tabla se rehízo control por control con evidencia. Desde entonces se cerraron fuzz y ZAP (#118), las pruebas de seguridad faltantes (#119), Dependabot, `trivy config` y gitleaks (#120), la firma con SBOM y provenance (#122), el respaldo con alarma (#123–#124), los errores ocultos de `crear.sh` (#125) y el WAF (PR E). La rúbrica nueva es más estricta en cuatro puntos que la anterior no separaba (TLS de origen, rotación, alertas de seguridad, respaldo fuera de la cuenta), por eso quedó en 77,5 % antes del WAF, 82,5 % con él, 87,5 % con el gobierno de GitHub y 90 % con las alertas de seguridad y CloudTrail.

- Lo que sigue a medias (DAST, protección de datos, rotación, registros, respaldos) y el TLS de origen dependen de staging, dominio propio y procesos (sección 3).

## 2. Controles implementados por capa

Mapa a [OWASP ASVS 4.0 nivel 2](asvs-l2-checklist.md) (capítulos V1–V14) y al [OWASP Top 10 2021](https://owasp.org/Top10/) (A01–A10). El detalle de cada requisito ASVS está en el checklist; aquí solo se agrupa.

### 2.1 Código (la API)

| Control | Dónde | ASVS | Top 10 |
|---|---|---|---|
| Argon2id 64 MB/3, política de contraseña, hash falso anti-enumeración, bloqueo 5 → 15 min fail-closed | `internal/auth` | V2 | A07 |
| JWT HS256 de 15 min, `iss`/`aud`, leeway 0, `alg:none` → 401, refresh rotado con hash SHA-256 y revocación por familia | `internal/auth/tokens.go`, `middleware/jwt.go` | V3 | A07, A02 |
| Google Sign-In con verificador real y revisión de `iss`, `email_verified` | `internal/auth/google.go` (nv-72) | V2 | A07 |
| Denegar por defecto, roles `consumidor/comerciante/administrador`, dueño validado en el service, sin ids ajenos en `/yo` | `middleware/roles.go`, services | V4 | A01 |
| Validación en la frontera, DTOs, `version` + `If-Match` en escrituras, cursores firmados | `internal/shared`, handlers | V5, V11, V13 | A03, A04 |
| Cifrado AES de campos sensibles con dato asociado (no se puede copiar entre cuentas); teléfono como HMAC-SHA-256 | `internal/platform/cifrado`, `internal/usuarios/cifrado.go`, `internal/negocios/expediente` | V6, V8 | A02 |
| Cabeceras, CSP `default-src 'none'`, HSTS, CORS de 1 origen; sin compresión en rutas con tokens (BREACH) | `middleware/security_headers.go`, `cors.go`, `compress.go` | V9, V14 | A05 |
| Rate limit antes de auth, 429 uniforme, límites por recurso sensible (CLABE, RFC, cupones) | `middleware/ratelimit.go`, `docs/rate-limit-matrix.md` | V11 | A04, A07 |
| Imágenes: validación de tipo y tamaño, moderación con Rekognition | `internal/imagenes`, `internal/moderacion` | V12 | A04 |
| Errores uniformes sin trazas en prod; logs sin secretos, correos ni credenciales de URI | `middleware/errors.go`, `internal/platform/logger` | V7 | A09 |
| Privacidad: consentimiento versionado, exportación y borrado a 30 días (ARCO) | `internal/privacidad`, `internal/usuarios` | V8 | A04 |
| Enlaces de un solo uso con el token en el fragmento; correo con STARTTLS y CR/LF rechazados | `internal/platform/mailer`, `tests/security/mailer_security_test.go` | V2, V9 | A03, A07 |

### 2.2 Integración continua (cada PR)

| Control | Dónde | ASVS | Top 10 |
|---|---|---|---|
| Semgrep, gosec, golangci-lint (`errcheck`, `gocyclo ≤ 10`) | `ci-pr.yml` `semgrep`, `go-security` | V10, V14 | A03, A05 |
| `govulncheck` y Dependabot semanal | `go-security`, `.github/dependabot.yml` | V14 | A06 |
| gitleaks sobre los commits | `guardrails` | V14 | A02, A05 |
| Dockle y Trivy de la imagen; `trivy config` de Terraform | `image-scan`, `iac-scan` | V14 | A05, A06 |
| Unit, Integration, Security, Database y Contract contra bases reales (Testcontainers) | `pipeline` | V1 | A01, A04 |
| Critic B1–B4 (orden del pipeline de middlewares, sin tokens en logs), test de arquitectura | `scripts/critic-guardrails.sh`, `internal/arch` | V1, V7 | A04, A09 |
| Semanal: fuzz real, ZAP pasivo, mutación completa | `nightly.yml` | V5 | A03, A05 |

### 2.3 Entrega continua

| Control | Dónde | ASVS | Top 10 |
|---|---|---|---|
| SBOM y provenance (SLSA) de buildx, firma cosign sin llaves; `id-token: write` solo en `firmar` | `cd-dev.yml` | V10, V14 | A08 |
| El servidor verifica la firma y la identidad antes de instalar; firma inválida → `failed_digest` | `infra/aws-dev/scripts/actualizar.sh` | V10 | A08 |
| Despliegue gradual (una instancia a la vez, nunca sale la única sana) | `actualizar.sh` | V1 | — |

### 2.4 Infraestructura

| Control | Dónde | ASVS | Top 10 |
|---|---|---|---|
| Deployer IAM con MFA y política acotada a `wherever-dev-*`; boundary para el rol de las instancias; raíz con MFA y sin claves | `infra/aws-dev/iam/`, README «Antes de empezar» | V1, V14 | A01, A05 |
| IMDSv2 obligatorio, EBS cifrado, sin puerto 22 (Systems Manager), 8080 solo desde el balanceador, balanceador solo desde CloudFront | `main.tf`, `alb.tf` | V14 | A05, A10 |
| HTTPS para el usuario (`redirect-to-https`) en API e imágenes | `cdn.tf`, `imagenes.tf` | V9 | A02 |
| Buckets privados, sin ACL, cifrados, solo TLS | `imagenes.tf`, `respaldos.tf` | V8, V14 | A01, A05 |
| Secretos en SSM SecureString; GitHub no tiene claves de AWS | `deploy-env.sh` | V6, V14 | A02, A05 |

### 2.5 Datos

| Control | Dónde | ASVS | Top 10 |
|---|---|---|---|
| Cifrado de campos y HMAC del teléfono (ver 2.1); llaves `DATA_KEYS` rotables con `make migrar-cifrado` | `internal/platform/cifrado` | V6 | A02 |
| Atlas con usuario propio del servidor (`readWrite` solo en `wherever`) y lista de IP | README paso 5 | V14 | A01 |
| Redis: todas las llaves con TTL; la contraseña nunca va a caché; estado de seguridad fail-closed | constitución §6 | V3 | A07 |
| Respaldo semanal cifrado y versionado; la instancia no puede leer ni borrar respaldos | `respaldos.tf`, `respaldo-mongo.sh` | V8 | A08 |
| Stripe Connect: el backend nunca ve números de tarjeta | `internal/pagos` | V8 | A02 |

### 2.6 Operación

| Control | Dónde | ASVS | Top 10 |
|---|---|---|---|
| Logs JSON en CloudWatch, alarmas por correo (SNS) | `observabilidad.tf`, `docs/observabilidad.md` | V7 | A09 |
| Modo mantenimiento y versión mínima de la app | README «Mantenimiento» | V14 | — |
| Restauración de MongoDB documentada (primero a un Mongo local) | README «Restaurar» | — | — |
| Presupuesto con aviso de AWS; `crear.sh`/`destruir.sh` con plan y confirmación escrita | `infra/aws-dev/scripts/` | — | A05 |

## 3. Brecha frente a una app bancaria o fintech

Wherever no custodia dinero: los pagos van por Stripe (Connect Express) y el reparto de gastos de «La Vaca» solo registra deudas. Aun así guarda datos personales, alergias, CLABE y RFC de negocios, por lo que la comparación con fintech es útil. Los costos son **estimaciones** en dólares, con precios públicos de lista a noviembre de 2026; el esfuerzo es de una persona.

| # | Brecha | Por qué importa | Esfuerzo | Costo (estimación) |
|---|---|---|---|---|
| 3.1 | **WAF** en CloudFront (reglas administradas de AWS, límite por IP) | Filtra inyección, bots y abuso antes de llegar a la API; con la cabecera secreta de origen, otra distribución de CloudFront ya no alcanza el balanceador | 1 día | ≈US$9–10 al mes (web ACL + 4 reglas + peticiones). **Hecho** (PR E, `waf.tf`) |
| 3.2 | **MFA para usuarios** (TOTP y, para comerciantes y administradores, obligatorio) | En banca es obligatorio; una contraseña robada hoy basta para entrar | 4–6 días | US$0 (TOTP); SMS cobra por mensaje |
| 3.3 | **Staging separado de producción** | Hoy un solo entorno sirve de prueba y de demostración; no hay dónde probar ataques ni migraciones sin riesgo | 2–3 días (Terraform ya está parametrizado) | US$15–40 al mes según tenga una o dos instancias |
| 3.4 | **DAST activo** (ZAP *full scan* o equivalente) contra staging | El pasivo solo mira respuestas; el activo intenta inyección, XSS y control de acceso | 1–2 días, depende de 3.3 | US$0 en GitHub Actions |
| 3.5 | **Prueba de penetración externa** | Encuentra fallas de lógica que las herramientas no ven; los reguladores y los clientes la piden cada año | 1–2 semanas del proveedor | US$3 000–15 000 por prueba |
| 3.6 | **TLS de CloudFront al balanceador** | Hoy ese tramo va en HTTP dentro de AWS; banca exige cifrado en todo el camino | 1 día tras el dominio | US$0 (certificado de ACM) |
| 3.7 | **Dominio propio y autenticación de correo** (SPF, DKIM, DMARC) | Sin dominio, Brevo marca el remitente gratuito como no conforme: los correos de recuperación pueden ir a spam y son fáciles de suplantar | 1 día | ≈US$12–20 al año de dominio + US$0,50 al mes de zona en Route 53 |
| 3.8 | **Plan de respuesta a incidentes** (roles, contactos, pasos para revocar sesiones y llaves, aviso a titulares) | Sin plan, una filtración se atiende improvisando; la LFPDPPP pide avisar a los titulares afectados | 2–3 días | US$0 |
| 3.9 | **Respaldos fuera de la cuenta y prueba de restauración periódica** | Una cuenta comprometida o `destruir.sh` borran también los respaldos; un respaldo que no se restaura no está probado | 1–2 días + 2 h al mes | <US$2 al mes (segunda cuenta con replicación S3 y Object Lock) |
| 3.10 | **Rotación de secretos** (JWT, `DATA_KEYS`, Atlas, Stripe, SMTP, Gemini) con calendario | Hoy se rota a mano y sin fecha; un secreto viejo filtrado sigue sirviendo | 2–4 días | US$0 manual con calendario; ≈US$4–6 al mes con Secrets Manager y rotación automática |
| 3.11 | **Registros de auditoría inmutables** (CloudTrail + S3 con Object Lock; colección de auditoría de acciones de administrador) | En fintech se debe poder probar quién hizo qué; los logs actuales duran 14 días y se pueden borrar | 3–5 días | US$1–5 al mes |
| 3.12 | **Alertas de seguridad** (picos de 401, 423, 429; cambios de IAM) | Antes las alarmas eran solo de disponibilidad | 1 día | <US$1 al mes. **Hecho** (`observabilidad.tf`, `auditoria.tf`) |
| 3.13 | **PCI DSS** | Stripe reduce el alcance: el backend nunca ve, guarda ni transmite tarjetas. Con los componentes de Stripe en la app el alcance suele ser el cuestionario SAQ A (confirmarlo con Stripe). No aplica la certificación completa | 1 día para el cuestionario | US$0 |
| 3.14 | **LFPDPPP** (ley vigente desde marzo de 2025) | Ya hay aviso de privacidad (`docs/aviso-privacidad.md`), consentimiento versionado, derechos ARCO y borrado. Falta: responsable de datos designado, contratos de encargado con los proveedores (Google, Stripe, Brevo, Firebase, AWS), registro de transferencias y procedimiento de vulneraciones (3.8) | 3–5 días con apoyo legal | Asesoría legal: variable |
| 3.15 | **SOC 2 / ISO 27001** como referencia | No se busca certificar; sirven como lista para ordenar políticas (accesos, cambios, proveedores, continuidad). Una certificación real cuesta decenas de miles de dólares al año | Referencia | US$20 000–60 000 al año si se certificara |
| 3.16 | **Gobierno del repositorio** (control 20) | Sin protección de `main`, un PR puede fusionarse con el CI en rojo; sin alertas, una vulnerabilidad nueva en una dependencia ya fusionada no avisa | 15 minutos del operador | US$0. **Hecho** (2026-11-19); falta exigir 1 aprobación cuando haya un segundo revisor |

## 4. Hoja de ruta

Ordenada por riesgo dentro de cada horizonte.

### Antes de producción (bloqueante)
1. ~~Gobierno de GitHub (3.16)~~: hecho el 2026-11-19. Falta subir el ruleset a 1 aprobación cuando haya un segundo revisor.
2. ~~WAF (3.1) y cabecera secreta de origen~~: hechos (PR E). Falta revisar sus métricas tras la primera semana y pasar a bloqueo `SizeRestrictions_BODY` si no hay cuerpos JSON legítimos de más de 8 KB.
3. Dominio propio, TLS hasta el balanceador y SPF/DKIM/DMARC (3.6, 3.7).
4. Staging separado (3.3) y DAST activo contra él (3.4).
5. Rotar todos los secretos que pasaron por el entorno de desarrollo y fijar calendario (3.10).
6. Plan de respuesta a incidentes (3.8) y obligaciones LFPDPPP pendientes (3.14).
7. Salir de los planes gratuitos de bases (sección 5).

### Primeros 3 meses
1. MFA para comerciantes y administradores; opcional para consumidores (3.2).
2. Respaldo fuera de la cuenta con Object Lock y una restauración probada al mes (3.9).
3. ~~CloudTrail y alertas de eventos de seguridad (3.12)~~: hechos. Falta Object Lock en el bucket de CloudTrail y la colección de auditoría de acciones de administrador (3.11).
4. Prueba de penetración externa (3.5) antes de crecer la base de usuarios.
5. Cuestionario PCI SAQ A con Stripe (3.13).

### Madurez
1. Rotación automática de secretos.
2. Políticas al estilo SOC 2 / ISO 27001 (accesos, cambios, proveedores, continuidad) y revisión anual (3.15).
3. Pentest anual y programa de reporte de vulnerabilidades (`SECURITY.md`).
4. Observabilidad completa (trazas y métricas) y simulacros de incidentes.

## 5. Riesgos aceptados

Aceptados para la etapa académica y piloto, **sin datos reales de clientes en volumen**. Cada uno se revisa antes de producción.

| Riesgo | Razón para aceptarlo | Mitigación actual |
|---|---|---|
| **MongoDB Atlas M0** (compartido, sin respaldos de Atlas, sin red privada) | Gratis; alcanza para el piloto | Respaldo propio semanal a S3 (#123), usuario de base exclusivo del servidor, lista de IP de Atlas, campos sensibles cifrados en la aplicación |
| **Neo4j Aura Free** se pausa a las 72 h sin uso y se borra a los 30 días en pausa | Gratis; el grafo se reconstruye con `make siembra` y los datos de origen viven en MongoDB | `/ready` da 503 y el actualizador no instala; revisar Aura antes de cada demostración |
| **Instancias apagadas de noche y en fines de semana** (22:00–08:00, lun–vie) | Ahorra la mayor parte del costo de cómputo | Encendido automático; el respaldo corre el viernes 21:00, antes del apagado |
| **Un solo entorno** (desarrollo que sirve de demostración) | Presupuesto y tiempo; no hay usuarios reales | Datos de prueba, ZAP solo pasivo, despliegue firmado y gradual; staging es el punto 4 de «antes de producción» |
| **Redis en Upstash Free** (500 000 comandos al mes) | Gratis | Sin cuota, lo autenticado falla cerrado (503) y `/ready` da `degraded`; presupuesto medido por pruebas |
| **Tramo CloudFront → balanceador en HTTP** | No hay dominio propio para el certificado | Tráfico dentro de AWS; balanceador limitado a CloudFront y 8080 limitado al balanceador |
