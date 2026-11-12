# NV-19 — Aceptación de términos y aviso de privacidad

## Contexto
La app debe registrar qué versión de términos y de aviso aceptó cada persona y volver a pedirla si cambia. **Origen**: TEC-29, TEC-30, US-001, US-003. **Depende de**: NV-16 (`TERMINOS_VERSION`, `AVISO_VERSION` y `GET /configuracion`) y NV-05 (privacidad del usuario). La dependencia con NV-18 (caché) no existe: esta spec no usa Redis.

## Requisitos
1. **Aceptación de términos (US-003):** `POST /api/v1/usuarios/yo/consentimientos` guarda la versión de términos y de aviso aceptadas, la fecha y la casilla opcional de promociones. Solo se aceptan las versiones vigentes (las de `GET /configuracion`); otra da 422.
2. El perfil (`GET /usuarios/yo`) y `PATCH /usuarios/yo/privacidad` devuelven `privacidad.consentimiento_vigente`: falso si la persona no aceptó las versiones vigentes, y la app vuelve a pedir la aceptación.
3. El aviso incluye la sección de terceros de `docs/aviso-privacidad.md`, con el proveedor externo de IA (Gemini, nv-26); un cambio en esa sección sube `AVISO_VERSION`.

## Diseño
- **Lo que ya existía (nv-05):** `usuarios.privacidad.version_terminos`, `fecha_aceptacion` y `promociones_correo`. Se reutilizan: la aceptación vigente se copia ahí y se agrega `privacidad.version_aviso` (vacío en cuentas previas, que quedan con `consentimiento_vigente` falso).
- **Historial:** colección `consentimientos` de solo agregar (`usuario_id`, `version_terminos`, `version_aviso`, `promociones_correo`, `fecha_aceptacion`), índice `ix_usuario_fecha`. Modelo, repositorio y ARCO en `internal/usuarios/consentimientos.go` (`ConsentRepository`, `ConsentPrivacy`); índice en `internal/indexes`.
- **Service y handler:** `internal/usuarios/account/consents.go` (`AcceptConsent`, `Versions`, `WithConsents`) y la ruta en `internal/usuarios/account/handler.go`, con `usuarios.RequireActive`. Sin `WithConsents` la ruta responde 503 (falla cerrado).
- **Versiones vigentes:** `config.VersionApp.Terminos` y `.Aviso` (nv-16), inyectadas en `cmd/api` y en `internal/testutil/authtest`.
- **ARCO:** "Descargar mis datos" incluye `consentimientos`; el borrado de la cuenta los borra; "Borrar mi historial" los conserva (son la prueba del consentimiento, no historial de uso).
- **Sin Redis ni caché:** no hay comandos nuevos; `TestPresupuestoRedis_*` no cambia.

## Contratos
| Método | Ruta | Auth | Respuestas |
|---|---|---|---|
| POST | `/api/v1/usuarios/yo/consentimientos` | JWT | 201, 400, 401, 403, 422 |

## Tests
- `Integration/Consentimientos` (`tests/integration/consentimientos_test.go`): aceptar, cambiar la versión y ver `consentimiento_vigente` falso, rechazar la versión vieja (422), aceptar de nuevo, historial intacto; 401 sin token y 422 sin cuerpo.
- `Unit` (`internal/usuarios/account/consents_test.go`): vigencia con las dos versiones, 422 antes de tocar la base, 503 sin repositorio, la exportación no lleva `consentimiento_vigente`.
- `Contract`: `usuarios/consentimientos_201` y el campo nuevo en los perfiles y en la exportación.
- `Database`: `consentimientos` con `ix_usuario_fecha`.

## Criterios
- Cumple los requisitos de arriba.

## Límites
- Sin límites propios: va con la política `global`.
- `PATCH /usuarios/yo/privacidad` con `version_terminos` sigue existiendo (nv-05) y no deja fila en el historial; la app debe usar la ruta nueva.
- El registro (`POST /usuarios`) guarda `version_terminos` pero no la del aviso: la cuenta nueva nace con `consentimiento_vigente` falso hasta llamar a la ruta nueva.

## Aprobación y Control de Cambios
- **Estado:** 🚧 Implementada (2026-11-12), pendiente de la firma del revisor. Local verde: build, vet, Unit, lint, critic, `check_sdd` y `check_endpoints`; Integration, Security, Database y Contract en el CI del PR.
- **Revisores:** Osmar Reséndiz (pendiente de aprobar)
- **Fecha:** —
- **Decisiones para el revisor:**
  1. Reutilizar la privacidad de nv-05 y agregar solo `version_aviso`; el historial va aparte en `consentimientos`.
  2. Solo se aceptan las versiones vigentes (422 si no).
  3. Mantener `version_terminos` en `PATCH /privacidad` y en el registro por compatibilidad con la app; retirarlos es otra spec.
  4. "Borrar mi historial" conserva los consentimientos; el borrado de la cuenta los elimina.
- **Detalle:** evidencia en `specs/nv-19/checklists/gates.md` + `Memoria.md`.
