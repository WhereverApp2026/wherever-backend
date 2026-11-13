# task.md — NV-62 Check-in y reseñas

## Fase 0 — Preparación
- [ ] T01 Spec revisada (decisiones del revisor) y filas de J7 en `docs/endpoints.md`.
- [ ] T02 Índices de `checkins`, `resenas` y `resenas_utiles` en `internal/indexes.All()`.

## Fase 1 — Implementación
- [ ] T03 `POST /checkins` por ubicación (100 m, precisión, salto imposible) y por `qr_checkin`; uno válido por día; `GET /checkins`.
- [ ] T04 «Por reseñar» y `POST /resenas` con check-in propio, una por visita y una cada 30 días por negocio; moderación de nv-42 y cola de nv-43.
- [ ] T05 Recálculo atómico de `calificacion` con `calificacion_suma` (semilla incluida) y borrado de `cache:negocio:{id}`.
- [ ] T06 Editar (48 h, If-Match), borrar lógico y «me fue útil».
- [ ] T07 Respuesta del negocio, sugerir respuesta con IA y resumen por periodo.
- [ ] T08 Reportes (3 ocultan) y lista pública con filtros, texto y orden.
- [ ] T09 Límites por usuario, `Purger` (anonimiza) y `Exporter`.

## Fase 2 — Cierre
- [ ] T10 Pruebas de las cinco suites, incluidas las de concurrencia.
- [ ] T11 Gates en `checklists/gates.md` y `Memoria.md`.
