# NV-15 — Correcciones de la auditoría

## T1 — Trabajo nocturno
- **Hacer**: `JOB_HOUR` con 3 por defecto; `Job.Start(hour)`; 21 en Parameter Store.
- **Verificar**: `TestLoad_JobHour`; en el servidor, que corra a las 21:00.

## T2 — Consentimiento
- **Hacer**: `privacidad.datos_sensibles`; 422 en gustos con alergias sin él; borrar alergias al retirarlo; ocultar las previas.
- **Verificar**: `TestConsentimiento_*`.

## T3 — Datos internos
- **Hacer**: `HIDE_INTERNALS` y `ExposeInternals()`; activado en Parameter Store.
- **Verificar**: `TestConfig_ExposeInternals`, `TestInternos_*`; en el servidor, sondas en 404.

## T4 — Contrato y documentos
- **Hacer**: `docs/endpoints.md`, `openapi.json`, capturas de contrato, `.env.example`, `Memoria.md`; avisar al equipo móvil.
- **Verificar**: `check_endpoints`, Contract.

## T5 — Despliegue
- **Hacer**: cargar los parámetros, abrir PR y comprobar tras fusionar.
- **Verificar**: `/health` con el SHA nuevo y `/ready` 200.
