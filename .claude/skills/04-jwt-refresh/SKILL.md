---
name: 04-jwt-refresh
description: JWT y refresh con rotación
---
# 04-jwt-refresh

> Rector: `.specify/memory/constitution.md`. Ante conflicto, manda la constitución.

## What I do
- JWT HS256 de 15 min, clave ≥32B por env, leeway 0, `iss/aud`, claims `sub/jti/roles/sid`. Roles `consumidor`, `comerciante`, `administrador`.
- Refresh de 30 días guardado como hash SHA-256 en `sesiones`, rota en cada uso y revoca la familia (`familia_id`) si se reutiliza. Logout → `jwt_revocado:{jti}`.

## Verify
- Token `alg:none` o sin firma → 401.
