# Spec Template — nv-0X (7 secciones + Aprobación)

> Constitución: `.specify/memory/constitution.md` §11. Sin cambio de contrato sin spec.

## 1. Objetivo
<!-- Qué módulo o slice cubre. Filosofía 00+01+02+03+04 liberable. -->

## 2. Alcance / No-alcance
<!-- In: spec→índices→DTO→servicio→handler→tests→gates→tag. Out: lo que esta spec no toca. -->

## 3. Contratos API
<!-- Rutas /api/v1/<recurso>, v1 estricta. Códigos 201/200/204/400/401/403/404/409/422/429/500. PageResult cursor. Enlazar docs/endpoints.md, no duplicar. -->

## 4. Modelo de datos
<!-- Colecciones en español plural, campos snake_case, creado_en/actualizado_en UTC, version, dinero en centavos. Índices por script. -->

## 5. Seguridad
<!-- Validar en frontera, JWT HS256 real + Argon2id real, rate limits, headers §6. -->

## 6. Tests / Gates
<!-- Unit, Integration con Testcontainers puerto fijo, Security, Database, Contract. Cobertura ≥45%. -->

## 7. Plan de entrega
<!-- go build, imagen, variables, curl /ready + login + 401 + headers, tag vX.Y.Z-modN. -->

## Aprobación
- [ ] Revisor 1 — fecha:
- [ ] CHECKLIST_PR.md OK
- [ ] Sin conflicto con constitución
