# NV-00 — Constitución base de Wherever Backend

## Contexto
Filosofía del proyecto aplicada a módulos liberables por slice, pero en esta etapa solo bases, sin ningún módulo de negocio. **Depende de**: —.

## Requisitos
1. Fijar el stack: `Go + Fiber + MongoDB + Neo4j + Redis + JWT + Argon2id`, sin `Terraform/AWS, CD, Stripe, FCM, OTel/Grafana completos, caos ni rendimiento` en esta etapa.
2. Exigir `spec-first + DoD por slice + trazabilidad spec↔código↔test + 1 revisor + Memoria.md`.
3. Fijar NFR verificables por comando: `timeout 60s, body 10MB, go build sin errores, cobertura >=45%`.
4. Registrar reglas no negociables: `sin secretos en Git, sin TODO sin issue, sin código muerto, inglés en código, estado sin globals, real-first sin fakes`.

## Diseño
- Principios: `stateless multi-instancia, seguridad primero, fail-fast, empírico-antes-teórico, documentar límites`.
- Stack:
  | Capa | Tecnología |
  |---|---|
  | Runtime | Go 1.22+ / Fiber v2 |
  | Persistencia | MongoDB + Neo4j + Redis, bases reales en integración |
  | Auth futuro | JWT HS256 + refresh rotativo + Argon2id |
  | Validación | go-playground/validator |
  | Tests | testing + testify + mockery (solo interfaces externas) + Testcontainers-go puerto fijo |
  | Logs | slog JSON + request_id |
  | SAST | Semgrep + gosec + golangci-lint + govulncheck |
  | CI | GitHub Actions solo `ci-pr.yml`, sin deploy |
- Naming: colecciones en español plural minúsculas, campos y JSON `snake_case`, `creado_en/actualizado_en` UTC, `version` para concurrencia, dinero en centavos.
- `AGENTS.md §1` cita este stack. `Memoria.md` inicial creada.

## Contratos
N/A — constitucional. Referencia normativa en `AGENTS.md §1`.

## Tests
N/A — verificación manual de estructura + `ConfigTest` (3): `.env.example válido + sin secretos + sin claves futuras`. `APP_ENV` admite solo `dev`, `staging` o `prod`.

## Criterios
- `AGENTS.md + Memoria.md` existen y citan el stack.
- Cada NFR tiene comando: `go build ./... → sin errores`, `scripts/check_coverage.sh --min 45 → >=45%`.
- `scripts/critic-guardrails.sh → PASS` en vacío.

## Límites
Sin `p95, gate de mutación, ZAP/RESTler bloqueantes, Trivy de imagen, /metrics completo, Pact real` — van como deuda documentada, no verde falso. Sin módulos de negocio, sin hosting.

## Aprobación y Control de Cambios
- **Estado:** 🚧 Borrador con evidencia (ejecutada, sin firma)
- **Revisores:** (pendiente de nombre)
- **Fecha:** 2026-11-02 (ejecución)
- **Detalle:** T1 ejecutada. Evidencia en `specs/nv-00/checklists/gates.md` + `Memoria.md`. Test real: `internal/platform/config/config_example_test.go` (`TestEnvExample_IsValid`, `TestEnvExample_HasNoSensitivePatterns`, `TestEnvExample_HasNoFutureKeys`). `scripts/check_coverage.sh` llega en `nv-03`; hasta entonces el NFR de cobertura queda documentado sin medir.
