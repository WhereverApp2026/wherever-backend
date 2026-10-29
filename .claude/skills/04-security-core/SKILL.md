---
name: 04-security-core
description: Argon2id, bloqueo y headers
---
# 04-security-core

> Rector: `.specify/memory/constitution.md`. Ante conflicto, manda la constitución.

## What I do
- Argon2id 64MB/3iter con formato PHC; `NeedsRehash`.
- Bloqueo 5 intentos → 15 min en `login_fallos:{correo}`, `423` con minutos restantes, fail-closed; cuenta también correos inexistentes.
- Headers outermost: CSP, HSTS no-dev, nosniff, DENY. La contraseña nunca en caché ni logs.

## Verify
- Hash real + bloqueo + headers probados.
