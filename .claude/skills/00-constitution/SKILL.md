---
name: 00-constitution
description: Constitución base sin negocio, stateless multi-instancia
---
# 00-constitution

> Rector: `.specify/memory/constitution.md`. Ante conflicto, manda la constitución.

## What I do
- Stack: Go + Fiber + MongoDB + Neo4j + Redis + JWT HS256 + Argon2id + validator.
- Fuera: negocio, Terraform/AWS, CD, caos, rendimiento.
- Principios: stateless multi-instancia, seguridad primero, fail-fast, empírico, real-first sin fakes.

## Cuando usarme
- Siempre. Cada módulo sale liberable con todo `00+01+02+03+04` aplicado.

## No hacer
- Sin módulos de negocio, sin caché de entidades, sin `/metrics` completo en esta etapa.

## Verify
- `go build ./...` sin errores.
