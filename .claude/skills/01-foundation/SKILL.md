---
name: 01-foundation
description: Pipeline final día 1 y arranque
---
# 01-foundation

> Rector: `.specify/memory/constitution.md`. Ante conflicto, manda la constitución.

## What I do
- Orden: SecurityHeaders outermost → ErrorHandler → RequestID → ProxyHeaders → CORS 1 origen → RateLimiter antes de Auth → JWT → Roles → Handlers.
- `main.go` mínimo; la app se construye en una función que los tests reutilizan. Body 10MB, timeout 60s.

## Cuando usarme
- Al crear `cmd/api/main.go` o tocar el pipeline. No tocar el pipeline sin el orden §4 de la constitución.

## Verify
- `go build ./...` + la app arranca en Integration/Security.
