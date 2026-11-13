# NV-43 — Cola de revisión y reglas de moderación

## Contexto
Lo dudoso espera a un administrador, que lo aprueba o rechaza y ajusta las reglas. **Origen**: TEC-31 y hoja A7 del Excel. **Depende de**: NV-42.

## Requisitos
1. Lo dudoso entra a **cuarentena** y no se publica hasta que un administrador lo apruebe o rechace con motivo.
2. Endpoints de administrador: cola, detalle con puntajes, aprobar o rechazar, ver y ajustar reglas (umbrales y palabras).
3. Cada decisión queda registrada con quién y cuándo.

## Diseño
- **Colecciones:** `moderacion_cola` (`tipo`, `referencia`, `autor_id`, `texto`, `imagen_clave`, `veredicto`, `puntajes`, `falla`, `estado` pendiente/aprobado/rechazado, `motivo`, `revisor_id`, `decidido_en`, `creado_en`, `version`) y `moderacion_reglas` (un solo documento `_id: "vigentes"` con `cuarentena`, `rechazo`, `palabras`, `version`, `actualizado_por`, `actualizado_en`). Índices en `moderacion.Indexes()` (lo corre `scripts/indexes` y `actualizar.sh` en cada despliegue): único parcial `tipo + referencia` mientras está pendiente, `estado + creado_en + _id` para la cola y `autor_id` para ARCO.
- **Entrada a la cola:** sin endpoint interno HTTP; en el monolito el `POST /interno/moderacion` del Excel no se expone. El llamador (nv-45) hace `Moderador.Evaluar` y, si el `Resultado` es `cuarentena`, `Service.Encolar(Entrada)`. Solo se encola cuarentena; la imagen va por su llave de S3 (nv-44), nunca en bytes. La decisión llega al módulo dueño del recurso por `Destino.AlDecidir`, registrado con `WithDestino(tipo, …)`; si el destino falla, la decisión queda guardada y el recurso sigue sin publicarse.
- **Reglas:** `Service` cumple `FuenteReglas`: el Moderador lee el documento en cada evaluación (una lectura por `_id`), así los umbrales cambian sin recompilar ni reiniciar. Sin documento se usan `ReglasPorDefecto()` (40 y 80). Si la base falla, `Evaluar` deja cuarentena (`FallaReglas`), como en nv-42.
- **Umbral mínimo:** `Reglas.Validar` exige `40 ≤ cuarentena < rechazo ≤ 100`. El 40 es `CuarentenaMinima = confianzaMinima` de Rekognition: un umbral menor no vería etiquetas nuevas en las imágenes.
- **Concurrencia:** `PATCH` de la cola y de las reglas exigen `If-Match` con la `version` (400 sin ella). Decidir es un `findOneAndUpdate` condicionado a `estado: pendiente` y la versión: la segunda decisión es 409.
- **Privacidad:** `moderacion.Privacy` borra y exporta los elementos del autor (ARCO). El id del administrador en la decisión se conserva.
- **Permisos de AWS:** `rekognition:DetectModerationLabels` ya está desde nv-42; la lectura del bucket es de nv-44. Esta spec no toca Terraform.
- **Configuración:** sin variables nuevas. El módulo HTTP se arma en `cmd/api` (solo Mongo y el cursor firmado que ya existe); el `Moderador` lo arma nv-45.

## Contratos
| Método | Ruta | Auth | Respuestas |
|---|---|---|---|
| GET | `/api/v1/admin/moderacion` | administrador | 200, 401, 403, 422 |
| GET | `/api/v1/admin/moderacion/{id}` | administrador | 200, 401, 403, 404 |
| PATCH | `/api/v1/admin/moderacion/{id}` | administrador, `If-Match` | 200, 400, 401, 403, 404, 409, 422 |
| GET | `/api/v1/admin/moderacion/reglas` | administrador | 200, 401, 403 |
| POST | `/api/v1/admin/moderacion/reglas` | administrador | 201, 401, 403, 409, 422 |
| PATCH | `/api/v1/admin/moderacion/reglas` | administrador, `If-Match` | 200, 400, 401, 403, 404, 409, 422 |

Detalle de cuerpos en `docs/endpoints.md`. Límite: política `admin` (200 por minuto por IP). El Excel no está en el repositorio; las filas salen de esta tabla, que copió la hoja A7.

## Tests
- `tests/integration/moderacion_test.go`: aprobar (quién y cuándo), rechazar con motivo, rechazar sin motivo (422), doble decisión (409), versión vieja (409), sin `If-Match` (400), reencolar el mismo recurso, cola y paginación, reglas (por defecto, crear, 409 al repetir, ajustar con versión, umbral 39 = 422) y que el Moderador usa las reglas ajustadas sin reiniciar; ARCO borra y exporta.
- `tests/security/moderacion_security_test.go`: las seis rutas dan 401 sin token y 403 a un consumidor.
- Unit (`internal/moderacion`): `Validar` rechaza cuarentena 39; validación de los DTO.

## Criterios
- Los umbrales se ajustan sin recompilar ni reiniciar.
- Nada en cuarentena se publica sin decisión de un administrador.

## Límites
- Hasta 200 palabras bloqueadas de 1 a 60 caracteres; motivo de hasta 500; página de hasta 50.

## Aprobación y Control de Cambios
- **Estado:** 🚧 Borrador con evidencia (2026-11-13), implementada en la rama `nv-43` (desde `nv-42`); ola 6
- **Revisores:** Osmar Reséndiz (pendiente de aprobar)
- **Fecha:** —
- **Drift corregido (2026-11-13):** las reglas usan un solo documento (POST crea, PATCH ajusta con `If-Match`, GET da las por defecto si no hay); los permisos de S3 salen de esta spec (son de nv-44); se agregan 400/401/403/422 a los contratos y el umbral mínimo de 40.
- **Detalle:** la evidencia irá en `specs/nv-43/checklists/gates.md` + `Memoria.md`.
