# NV-45 — Subida, validación y publicación de imágenes

## Contexto
La persona sube una foto directo a S3; la API la valida, la modera y la publica sin metadatos. **Origen**: TEC-32. **Depende de**: NV-42 (moderación), NV-43 (cola de revisión y reglas) y NV-44 (bucket privado y CloudFront).

## Requisitos
1. `POST /imagenes/subida` devuelve un formulario POST firmado de S3 con límites: JPEG, PNG o WebP, de 1 byte a 5 MB.
2. La imagen cae en el prefijo `cuarentena/` (vence a 1 día por la regla del bucket).
3. Al confirmar, la API valida el tipo real y el tamaño, la **vuelve a codificar** en JPEG (quita EXIF y GPS, respeta la orientación) en dos tamaños (1080 y 320 px de lado mayor, sin ampliar) y la modera una sola vez: aprobada se publica en `i/{id}/`, rechazada se borra y la dudosa queda `en_revision` y entra a la cola de nv-43; la decisión del administrador la publica o la borra.
4. Al eliminar la cuenta se borran sus imágenes (ARCO, `Purger`) y se exportan sus filas (`Exporter`). Las del negocio quedan para nv-55/56, que aún no existen.
5. Límite por usuario: 20 formularios por hora y 50 por día.

## Diseño
- **Formulario firmado** con política (clave exacta, `Content-Type` exacto, `content-length-range` 1..5 MB) firmada con el rol de la instancia; vence en 5 minutos.
- **Procesamiento** en la API con `image` de Go y `golang.org/x/image` (WebP y escalado); máximo 6000 px por lado y 64 MB decodificada, calculado con la cabecera antes de decodificar (bombas, JPEG progresivo y PNG entrelazado cuentan más). Una decodificación a la vez por instancia.
- **Moderación** con el `Moderador` de nv-42; los umbrales salen de `moderacion_reglas` en cada evaluación (nv-43). Una falla del proveedor deja `en_revision` sin encolar y la persona puede volver a confirmar.
- **Colección `imagenes`:** `_id` aleatorio de 32 hex, `dueno_id`, `tipo`, `estado` (`pendiente`, `procesando`, `en_revision`, `publicada`, `rechazada`), `motivo`, `moderacion`, fechas y `expira_en` (TTL de las pendientes, 24 h).
- **Direcciones:** `{IMAGENES_URL}/i/{id}/1080.jpg` y `/320.jpg`, caché inmutable de un año.
- **Configuración:** `IMAGENES_BUCKET` e `IMAGENES_URL` van juntas (obligatorias en prod; sin ellas las rutas responden 503) e `IMAGENES_REGION` (por defecto `us-east-2`). El `hop_limit` de IMDSv2 sube a 2 para que el contenedor tome las credenciales del rol.

## Contratos
| Método | Ruta | Auth | Respuestas |
|---|---|---|---|
| POST | `/api/v1/imagenes/subida` | JWT | 201, 400, 401, 403, 422, 429, 503 |
| POST | `/api/v1/imagenes/{id}/confirmar` | JWT dueño | 200, 401, 403, 404, 409, 422, 503 |
| DELETE | `/api/v1/imagenes/{id}` | JWT dueño | 204, 401, 403, 404, 503 |

## Tests
- Unit `internal/imagenes`: `proceso_test.go` (JPEG, PNG con transparencia, WebP con y sin pérdida, no amplía, extensión mentirosa, no imagen, bomba por cabecera, EXIF con GPS se quita y respeta la orientación, JPEG truncado) y `almacen_test.go` (la política firmada lleva clave, tipo, 1..5 MB y 5 minutos). `config_test.go` para las variables.
- Integration `tests/integration/imagenes_test.go` con el doble del bucket (`imagenestest`, S3 es interfaz externa) y el Rekognition local (`rekognitiontest`): subida → moderación → publicada; confirmar sin subir (409); tipo equivocado o archivo inválido (422); moderación rechaza o deja dudosa; moderación caída y reintento; límite por hora; borrar la cuenta borra sus imágenes; sin configuración 503; la cola de nv-43 publica o descarta.
- Security `tests/security/imagenes_security_test.go`: sin sesión 401; el dueño ajeno no confirma ni borra (404).
- Que el bucket no sea público y que `cuarentena/` no salga por CloudFront lo prueba `verificar.sh --imagenes` de nv-44.
- En vivo (tras el despliegue): subir una foto real, verla por CloudFront y comprobar que no tiene EXIF.

## Criterios
- Las fotos publicadas no conservan metadatos.
- Nada sin moderación aprobada (automática o del administrador) se sirve.

## Límites
- Sin edición ni recorte en la API.
- Solo fotos; sin video.
- Un original en revisión vence a 1 día: si el administrador decide después, la foto queda en revisión sin publicarse y la persona la sube de nuevo.

## Aprobación y Control de Cambios
- **Estado:** 🚧 Borrador con evidencia (2026-11-13): implementada; local verde (build, vet, Unit, lint, critic, `check_sdd`, `check_endpoints`); Integration, Security, Database y Contract en el CI del PR. Falta la firma y la prueba en vivo.
- **Revisores:** Osmar Reséndiz (pendiente de aprobar)
- **Fecha:** —
- **Decisiones tomadas:** 6000 px y 64 MB decodificada en lugar de 4000 px (fotos de teléfono actuales); tope diario de 50 para acotar Rekognition; `AVISO_VERSION` sin subir por indicación del revisor.
- **Detalle:** evidencia en `specs/nv-45/checklists/gates.md` + `Memoria.md`.
