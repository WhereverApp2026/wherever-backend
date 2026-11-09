# NV-15 — Correcciones de la auditoría: trabajo nocturno, consentimiento y datos internos

## Contexto
La auditoría del 5 de noviembre de 2026 contra los requerimientos, el documento técnico y el Excel de historias encontró tres fallas que afectan hoy al servidor público. Esta spec las corrige sin agregar módulos. **Depende de**: NV-05 (privacidad), NV-07 (servidor con horario) y NV-08 (CD).

## Requisitos
1. **Trabajo nocturno.** La hora es configurable con `JOB_HOUR` (0 a 23, `America/Mexico_City`; 3 por defecto). En AWS es 21: la instancia se apaga a las 22:00, y a las 03:00 el trabajo nunca corría, por lo que las exportaciones no se generaban y las cuentas no se borraban.
2. **Consentimiento (US-010).** `privacidad.datos_sensibles` nace en `false`. Con `false`, `PUT /usuarios/yo/gustos` con `alergias` no vacío responde 422 con el campo `alergias` y no guarda nada. Sin alergias no hace falta.
3. Al pasar `datos_sensibles` a `false` se borran las alergias guardadas. Las alergias guardadas antes de esta spec no se muestran mientras no haya consentimiento.
4. **Datos internos.** Con `HIDE_INTERNALS=true`, fuera de prod, no se muestra `detalle` y `/api/v1/probe/*` da 404. En prod siguen siempre ocultos. El servidor de AWS lo activa. Dev y las pruebas de staging no cambian.
5. `docs/endpoints.md` deja de decir que `/ready` es de red interna: en el servidor actual es público, sin direcciones.

## Diseño
- `config.Config` suma `JobHour` y `HideInternals`; `ExposeInternals()` reemplaza a `!IsProd()` en el manejador de errores y en el registro de sondas. `/docs` sigue solo en dev.
- `privacidad.Job.Start(hour)` recibe la hora; `cmd/api` la toma de la configuración.
- `JOB_HOUR` y `HIDE_INTERNALS` van en Parameter Store como `String`, igual que `APP_ENV` y `TRUSTED_PROXIES`, y quedan listados en el README de `infra/aws-dev`. `actualizar.sh` corre `deploy-env.sh` antes de cambiar la API, así que se aplican solos con la siguiente imagen, sin SSH.
- Campo nuevo y no reutilización de `estado_animo`: ese interruptor es del ánimo y una alergia no es un ánimo.

## Contratos
- `privacidad` suma `datos_sensibles` (booleano) en `PATCH /usuarios/yo/privacidad` y en el perfil.
- **Cambio para la app:** `PUT /usuarios/yo/gustos` con `alergias` responde 422 si antes no se aceptó `datos_sensibles`.
- `docs/endpoints.md`, `internal/http/openapi.json` y las capturas de contrato se actualizan.

## Tests
- Unit: `TestLoad_JobHour`, `TestConfig_ExposeInternals`, `TestLoad_HideInternals`.
- Integration: `TestConsentimiento_*` (sin consentimiento, sin alergias, con él, retirarlo, retirarlo sin gustos) y `TestInternos_*`.
- Contract: captura regenerada, revisada y sin otro cambio.
- Real: en el servidor, `/api/v1/probe/error` da 404, y a las 21:00 corre el trabajo.

## Criterios
- Cinco suites, lint, Semgrep, `critic`, `check_sdd` y `check_endpoints` en verde; cobertura ≥ 45 %.
- Los errores del servidor público no traen `detalle`.
- Sin secretos nuevos.

## Límites
- **Fines de semana:** la instancia está apagada, así que el trabajo no corre sábado ni domingo y lo pendiente espera al lunes. El plazo de 30 días de borrado puede pasarse hasta 2 días.
- El correo sigue sin salir (hallazgo C4): la exportación se genera, pero el aviso solo queda en el log hasta elegir un proveedor.
- `estado_animo` aún no guarda ningún dato: cuando exista, deberá respetar este mismo tipo de consentimiento.
- La app debe pedir `datos_sensibles` antes de mandar alergias; hasta entonces recibirá 422 si las manda.
- Las alergias ya guardadas sin consentimiento se ocultan, pero siguen en la base hasta que la cuenta las reemplace o se retire el consentimiento.
- No corrige el resto de la auditoría.

## Aprobación y Control de Cambios
- **Estado:** 🚧 Implementada en `main` (PR 20) y comprobada en el servidor; sin firmar
- **Revisores:** Osmar Reséndiz (pendiente de firmar)
- **Fecha:** 2026-11-05 (ejecución)
- **Detalle:** se numera nv-15 porque nv-11 a nv-14 ya tienen otro alcance. La evidencia va en `specs/nv-15/checklists/gates.md` + `Memoria.md`.
