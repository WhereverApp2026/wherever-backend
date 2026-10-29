---
name: 05-cache-redis
description: Redis: límites, candados, idempotencia, revocados
---
# 05-cache-redis

> Rector: `.specify/memory/constitution.md`. Ante conflicto, manda la constitución.

## What I do
- Prefijos (constitución §6): `jwt_revocado:`, `login_fallos:`, `rate:`, `idem:`, `lock:`, `cuota:`, `cache:`. Todo con TTL.
- Tiempos de espera cortos. Si Redis falla: solo `rate:` pasa a memoria por instancia (con aviso); el resto es fail-closed (`503`).

## No hacer
- Sin `cache:{entidad}` en esta etapa.

## Verify
- Prueba con Redis caído: la API responde, `/ready` marca `degraded` y las operaciones fail-closed dan `503`.
